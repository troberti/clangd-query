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

	"clangd-query/internal/clangd"
	"clangd-query/internal/commands"
	"clangd-query/internal/logger"
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
	clangdClient  *clangd.ClangdClient
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
	ID     interface{}            `json:"id"`
	Method string                 `json:"method"`
	Params map[string]interface{} `json:"params,omitempty"`
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

	daemon.logger.Info("Starting daemon for project: %s", config.ProjectRoot)

	// Check for existing daemon
	if err := daemon.checkExistingDaemon(); err != nil {
		daemon.logger.Error("Error checking existing daemon: %v", err)
		os.Exit(1)
	}

	// Write lock file
	if err := WriteLockFile(config.ProjectRoot, os.Getpid(), daemon.socketPath); err != nil {
		daemon.logger.Error("Failed to write lock file: %v", err)
		os.Exit(1)
	}
	defer RemoveLockFile(config.ProjectRoot)

	// Ensure compilation database exists
	buildDir, err := EnsureCompilationDatabase(config.ProjectRoot, daemon.logger)
	if err != nil {
		daemon.logger.Error("Failed to find compilation database: %v", err)
		os.Exit(1)
	}

	// Start clangd
	daemon.logger.Info("Starting clangd with build directory: %s", buildDir)
	daemon.clangdClient, err = clangd.NewClangdClient(config.ProjectRoot, buildDir, daemon.logger)
	if err != nil {
		daemon.logger.Error("Failed to start clangd: %v", err)
		os.Exit(1)
	}
	defer daemon.clangdClient.Stop()

	// Setup file watcher
	daemon.fileWatcher, err = NewFileWatcher(config.ProjectRoot, daemon.onFilesChanged, daemon.logger)
	if err != nil {
		daemon.logger.Error("Failed to setup file watcher: %v", err)
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
		daemon.logger.Error("Failed to start socket server: %v", err)
		os.Exit(1)
	}

	daemon.logger.Info("Daemon started successfully")

	// Wait for shutdown
	<-daemon.shutdown

	daemon.logger.Info("Daemon shutting down")
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

func (d *Daemon) checkExistingDaemon() error {
	lockInfo, err := ReadLockFile(d.projectRoot)
	if err != nil {
		return err
	}

	if lockInfo != nil {
		if IsProcessAlive(lockInfo.PID) {
			if IsDaemonStale(lockInfo) {
				d.logger.Info("Existing daemon is stale, attempting to stop it")
				// Try to gracefully stop the old daemon
				syscall.Kill(lockInfo.PID, syscall.SIGTERM)
				time.Sleep(100 * time.Millisecond)
			} else {
				return fmt.Errorf("daemon already running with PID %d", lockInfo.PID)
			}
		} else {
			d.logger.Debug("Found stale lock file, cleaning up")
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
		d.logger.Info("Idle timeout reached, shutting down")
		close(d.shutdown)
	})
}

func (d *Daemon) setupSignalHandlers() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGTERM, syscall.SIGINT)

	go func() {
		sig := <-sigChan
		d.logger.Info("Received signal: %v", sig)
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
				d.logger.Error("Error accepting connection: %v", err)
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

	d.logger.Info("Client %d connected", clientID)

	decoder := json.NewDecoder(conn)
	encoder := json.NewEncoder(conn)

	for {
		var req Request
		if err := decoder.Decode(&req); err != nil {
			if err.Error() != "EOF" {
				d.logger.Error("Client %d: Error decoding request: %v", clientID, err)
			}
			break
		}

		d.mu.Lock()
		d.totalRequests++
		d.mu.Unlock()

		d.logger.Info("Client %d: Request %s", clientID, req.Method)

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
			d.logger.Error("Client %d: Error encoding response: %v", clientID, err)
			break
		}
	}

	d.logger.Info("Client %d disconnected", clientID)
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
	}

	// All other commands go to clangd
	input, _ := req.Params["symbol"].(string)

	limit := -1
	if l, ok := req.Params["limit"].(float64); ok {
		limit = int(l)
	}

	var output string
	var err error

	switch req.Method {
	case "search":
		output, err = commands.Search(d.clangdClient, input, limit, d.logger)
	case "show":
		output, err = commands.Show(d.clangdClient, input, d.logger)
	case "view":
		output, err = commands.View(d.clangdClient, input, d.logger)
	case "usages":
		output, err = commands.Usages(d.clangdClient, input, limit, d.logger)
	case "hierarchy":
		output, err = commands.Hierarchy(d.clangdClient, input, limit, d.logger)
	case "signature":
		output, err = commands.Signature(d.clangdClient, input, d.logger)
	case "interface":
		output, err = commands.Interface(d.clangdClient, input, d.logger)
	default:
		return nil, fmt.Errorf("unknown method: %s", req.Method)
	}

	if err != nil {
		d.logger.Error("%s failed: %v", req.Method, err)
		return nil, err
	}

	return json.Marshal(map[string]string{"output": output})
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

func (d *Daemon) onFilesChanged(files []string) {
	d.logger.Debug("Files changed: %v", files)

	if d.clangdClient != nil {
		// Notify clangd about file changes
		d.clangdClient.OnFilesChanged(files)
	}
}
