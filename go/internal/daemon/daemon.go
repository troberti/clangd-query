package daemon

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"clangd-query/internal/commands"
	"clangd-query/internal/logger"
	"clangd-query/internal/lsp"
)

// Config contains daemon configuration
type Config struct {
	ProjectRoot string
	Verbose     bool
}

// Daemon manages the clangd process and handles client connections
type Daemon struct {
	projectRoot   string
	socketPath    string
	logger        logger.Logger
	clangdClient  *lsp.ClangdClient
	fileWatcher   *FileWatcher
	listener      net.Listener
	idleTimer     *time.Timer
	idleTimeout   time.Duration
	mu            sync.Mutex
	shutdown      chan struct{}
	connections   int
	totalRequests int
	startTime     time.Time
}

// Request represents a client request
type Request struct {
	ID      interface{}            `json:"id"`
	Method  string                 `json:"method"`
	Params  map[string]interface{} `json:"params,omitempty"`
}

// Response represents a daemon response
type Response struct {
	ID     interface{}     `json:"id"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  *ErrorResponse  `json:"error,omitempty"`
}

// ErrorResponse represents an error response
type ErrorResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// Run starts the daemon
func Run(config *Config) {
	daemon := &Daemon{
		projectRoot: config.ProjectRoot,
		socketPath:  GetSocketPath(config.ProjectRoot),
		shutdown:    make(chan struct{}),
		startTime:   time.Now(),
	}

	// Setup logging with config
	if err := daemon.setupLogging(config.Verbose); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to setup logging: %v\n", err)
		os.Exit(1)
	}
	defer func() {
		if fileLogger, ok := daemon.logger.(*logger.FileLogger); ok {
			fileLogger.Close()
		}
	}()

	daemon.log("Starting daemon for project: %s", config.ProjectRoot)

	// Check for existing daemon
	if err := daemon.checkExistingDaemon(); err != nil {
		daemon.log("Error checking existing daemon: %v", err)
		os.Exit(1)
	}

	// Write lock file
	if err := WriteLockFile(config.ProjectRoot, os.Getpid(), daemon.socketPath); err != nil {
		daemon.log("Failed to write lock file: %v", err)
		os.Exit(1)
	}
	defer RemoveLockFile(config.ProjectRoot)

	// Ensure compilation database exists
	buildDir, err := EnsureCompilationDatabase(config.ProjectRoot, daemon.logger)
	if err != nil {
		daemon.log("Failed to ensure compilation database: %v", err)
		os.Exit(1)
	}

	// Start clangd
	daemon.log("Starting clangd with build directory: %s", buildDir)
	daemon.clangdClient, err = lsp.NewClangdClient(config.ProjectRoot, buildDir, daemon.logger)
	if err != nil {
		daemon.log("Failed to start clangd: %v", err)
		os.Exit(1)
	}
	defer daemon.clangdClient.Stop()

	// Setup file watcher
	daemon.fileWatcher, err = NewFileWatcher(config.ProjectRoot, daemon.onFilesChanged, daemon.logger)
	if err != nil {
		daemon.log("Failed to setup file watcher: %v", err)
		// Continue without file watching
	} else {
		defer daemon.fileWatcher.Stop()
	}

	// Setup idle timeout
	daemon.setupIdleTimeout()

	// Setup signal handlers
	daemon.setupSignalHandlers()

	// Start socket server
	if err := daemon.startSocketServer(); err != nil {
		daemon.log("Failed to start socket server: %v", err)
		os.Exit(1)
	}

	daemon.log("Daemon started successfully")

	// Wait for shutdown
	<-daemon.shutdown

	daemon.log("Daemon shutting down")
}

func (d *Daemon) setupLogging(verbose bool) error {
	logPath := GetLogPath(d.projectRoot)
	
	// Determine file log level based on verbose flag
	fileLogLevel := logger.LevelInfo
	if verbose {
		fileLogLevel = logger.LevelDebug
	}
	
	// Create file logger
	fileLogger, err := logger.NewFileLogger(logPath, fileLogLevel)
	if err != nil {
		return err
	}
	
	d.logger = fileLogger
	return nil
}

func (d *Daemon) log(format string, args ...interface{}) {
	d.logger.Info(format, args...)
}

func (d *Daemon) checkExistingDaemon() error {
	lockInfo, err := ReadLockFile(d.projectRoot)
	if err != nil {
		return err
	}

	if lockInfo != nil {
		if IsProcessAlive(lockInfo.PID) {
			if IsDaemonStale(lockInfo) {
				d.log("Existing daemon is stale, attempting to stop it")
				// Try to gracefully stop the old daemon
				syscall.Kill(lockInfo.PID, syscall.SIGTERM)
				time.Sleep(100 * time.Millisecond)
			} else {
				return fmt.Errorf("daemon already running with PID %d", lockInfo.PID)
			}
		} else {
			d.log("Found stale lock file, cleaning up")
		}

		// Clean up old socket
		CleanupSocket(lockInfo.SocketPath)
		RemoveLockFile(d.projectRoot)
	}

	return nil
}

func (d *Daemon) setupIdleTimeout() {
	// Get timeout from environment or use default (30 minutes)
	timeoutStr := os.Getenv("CLANGD_DAEMON_TIMEOUT")
	if timeoutStr == "" {
		d.idleTimeout = 30 * time.Minute
	} else {
		if timeout, err := time.ParseDuration(timeoutStr); err == nil {
			d.idleTimeout = timeout
		} else {
			d.idleTimeout = 30 * time.Minute
		}
	}

	d.resetIdleTimer()
}

func (d *Daemon) resetIdleTimer() {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.idleTimer != nil {
		d.idleTimer.Stop()
	}

	d.idleTimer = time.AfterFunc(d.idleTimeout, func() {
		d.log("Idle timeout reached, shutting down")
		close(d.shutdown)
	})
}

func (d *Daemon) setupSignalHandlers() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGTERM, syscall.SIGINT)

	go func() {
		sig := <-sigChan
		d.log("Received signal: %v", sig)
		close(d.shutdown)
	}()
}

func (d *Daemon) startSocketServer() error {
	// Remove old socket if it exists
	CleanupSocket(d.socketPath)

	// Create socket listener
	listener, err := net.Listen("unix", d.socketPath)
	if err != nil {
		return err
	}
	d.listener = listener

	// Start accepting connections
	go d.acceptConnections()

	return nil
}

func (d *Daemon) acceptConnections() {
	defer d.listener.Close()
	defer CleanupSocket(d.socketPath)

	for {
		conn, err := d.listener.Accept()
		if err != nil {
			select {
			case <-d.shutdown:
				return
			default:
				d.log("Error accepting connection: %v", err)
				continue
			}
		}

		d.resetIdleTimer()
		go d.handleConnection(conn)
	}
}

func (d *Daemon) handleConnection(conn net.Conn) {
	defer conn.Close()

	d.mu.Lock()
	d.connections++
	clientID := d.connections
	d.mu.Unlock()

	d.log("Client %d connected", clientID)

	decoder := json.NewDecoder(conn)
	encoder := json.NewEncoder(conn)

	for {
		var req Request
		if err := decoder.Decode(&req); err != nil {
			if err.Error() != "EOF" {
				d.log("Client %d: Error decoding request: %v", clientID, err)
			}
			break
		}

		d.mu.Lock()
		d.totalRequests++
		d.mu.Unlock()

		d.log("Client %d: Request %s", clientID, req.Method)

		// Handle the request
		result, err := d.handleRequest(req)

		// Send response
		resp := Response{
			ID: req.ID,
		}

		if err != nil {
			resp.Error = &ErrorResponse{
				Code:    -1,
				Message: err.Error(),
			}
		} else {
			resp.Result = result
		}

		if err := encoder.Encode(resp); err != nil {
			d.log("Client %d: Error encoding response: %v", clientID, err)
			break
		}
	}

	d.log("Client %d disconnected", clientID)
}

func (d *Daemon) handleRequest(req Request) (json.RawMessage, error) {
	switch req.Method {
	case "status":
		return d.handleStatus()
	case "logs":
		return d.handleLogs(req)
	case "shutdown":
		go func() {
			time.Sleep(100 * time.Millisecond)
			close(d.shutdown)
		}()
		return json.Marshal(map[string]string{"status": "shutting down"})
	case "search", "show", "view", "usages", "hierarchy", "signature", "interface":
		// These will be forwarded to clangd
		return d.forwardToClangd(req)
	default:
		return nil, fmt.Errorf("unknown method: %s", req.Method)
	}
}

func (d *Daemon) handleStatus() (json.RawMessage, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	status := map[string]interface{}{
		"pid":           os.Getpid(),
		"projectRoot":   d.projectRoot,
		"uptime":        time.Since(d.startTime).String(),
		"totalRequests": d.totalRequests,
		"connections":   d.connections,
		"idleTimeout":   d.idleTimeout.String(),
	}

	return json.Marshal(status)
}

func (d *Daemon) handleLogs(req Request) (json.RawMessage, error) {
	// Get log level from params (default to INFO and above)
	minLevel := logger.LevelInfo
	if level, ok := req.Params["level"].(string); ok {
		switch level {
		case "error":
			minLevel = logger.LevelError
		case "verbose", "debug":
			minLevel = logger.LevelDebug
		default:
			minLevel = logger.LevelInfo
		}
	}
	
	// Get filtered logs from memory
	logs := d.logger.GetLogs(minLevel)
	return json.Marshal(map[string]string{"logs": logs})
}

func (d *Daemon) forwardToClangd(req Request) (json.RawMessage, error) {
	// Extract parameters
	params := req.Params
	
	// Get common parameters
	limit := -1
	if l, ok := params["limit"].(float64); ok {
		limit = int(l)
	}
	
	// Handle each command based on method
	switch req.Method {
	case "search":
		query, _ := params["query"].(string)
		if query == "" {
			d.logger.Error("search command called without query parameter")
			return nil, fmt.Errorf("search requires a query parameter")
		}
		
		result, err := commands.Search(d.clangdClient, query, limit, d.logger)
		if err != nil {
			d.logger.Error("search command failed: %v", err)
			return nil, err
		}
		// Return the formatted string wrapped in JSON
		return json.Marshal(map[string]string{"output": result})
		
	case "show":
		symbol, _ := params["symbol"].(string)
		if symbol == "" {
			d.logger.Error("show command called without symbol parameter")
			return nil, fmt.Errorf("show requires a symbol parameter")
		}
		
		result, err := commands.Show(d.clangdClient, symbol, d.logger)
		if err != nil {
			d.logger.Error("show command failed: %v", err)
			return nil, err
		}
		return json.Marshal(map[string]string{"output": result})
		
	case "view":
		symbol, _ := params["symbol"].(string)
		if symbol == "" {
			d.logger.Error("view command called without symbol parameter")
			return nil, fmt.Errorf("view requires a symbol parameter")
		}
		
		result, err := commands.View(d.clangdClient, symbol, d.logger)
		if err != nil {
			d.logger.Error("view command failed: %v", err)
			return nil, err
		}
		return json.Marshal(map[string]string{"output": result})
		
	case "usages":
		symbol, _ := params["symbol"].(string)
		if symbol == "" {
			d.logger.Error("usages command called without symbol parameter")
			return nil, fmt.Errorf("usages requires a symbol parameter")
		}
		
		result, err := commands.Usages(d.clangdClient, symbol, limit, d.logger)
		if err != nil {
			d.logger.Error("usages command failed: %v", err)
			return nil, err
		}
		return json.Marshal(map[string]string{"output": result})
		
	case "hierarchy":
		symbol, _ := params["symbol"].(string)
		if symbol == "" {
			d.logger.Error("hierarchy command called without symbol parameter")
			return nil, fmt.Errorf("hierarchy requires a symbol parameter")
		}
		
		result, err := commands.Hierarchy(d.clangdClient, symbol, limit, d.logger)
		if err != nil {
			d.logger.Error("hierarchy command failed: %v", err)
			return nil, err
		}
		return json.Marshal(map[string]string{"output": result})
		
	case "signature":
		symbol, _ := params["symbol"].(string)
		if symbol == "" {
			d.logger.Error("signature command called without symbol parameter")
			return nil, fmt.Errorf("signature requires a symbol parameter")
		}
		
		result, err := commands.Signature(d.clangdClient, symbol, d.logger)
		if err != nil {
			d.logger.Error("signature command failed: %v", err)
			return nil, err
		}
		return json.Marshal(map[string]string{"output": result})
		
	case "interface":
		symbol, _ := params["symbol"].(string)
		if symbol == "" {
			d.logger.Error("interface command called without symbol parameter")
			return nil, fmt.Errorf("interface requires a symbol parameter")
		}
		
		result, err := commands.Interface(d.clangdClient, symbol, d.logger)
		if err != nil {
			d.logger.Error("interface command failed: %v", err)
			return nil, err
		}
		return json.Marshal(map[string]string{"output": result})
		
	default:
		d.logger.Error("unknown command: %s", req.Method)
		return nil, fmt.Errorf("unknown command: %s", req.Method)
	}
}

func (d *Daemon) onFilesChanged(files []string) {
	d.log("Files changed: %v", files)
	
	if d.clangdClient != nil {
		// Notify clangd about file changes
		d.clangdClient.OnFilesChanged(files)
	}
}