package daemon

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/firi/clangd-query/internal/commands"
	"github.com/firi/clangd-query/internal/lsp"
)

// Daemon manages the clangd process and handles client connections
type Daemon struct {
	projectRoot   string
	socketPath    string
	logFile       *os.File
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
func Run(projectRoot string) {
	daemon := &Daemon{
		projectRoot: projectRoot,
		socketPath:  GetSocketPath(projectRoot),
		shutdown:    make(chan struct{}),
		startTime:   time.Now(),
	}

	// Setup logging
	if err := daemon.setupLogging(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to setup logging: %v\n", err)
		os.Exit(1)
	}
	defer daemon.logFile.Close()

	daemon.log("Starting daemon for project: %s", projectRoot)

	// Check for existing daemon
	if err := daemon.checkExistingDaemon(); err != nil {
		daemon.log("Error checking existing daemon: %v", err)
		os.Exit(1)
	}

	// Write lock file
	if err := WriteLockFile(projectRoot, os.Getpid(), daemon.socketPath); err != nil {
		daemon.log("Failed to write lock file: %v", err)
		os.Exit(1)
	}
	defer RemoveLockFile(projectRoot)

	// Ensure compilation database exists
	buildDir, err := EnsureCompilationDatabase(projectRoot)
	if err != nil {
		daemon.log("Failed to ensure compilation database: %v", err)
		os.Exit(1)
	}

	// Start clangd
	daemon.log("Starting clangd with build directory: %s", buildDir)
	daemon.clangdClient, err = lsp.NewClangdClient(projectRoot, buildDir)
	if err != nil {
		daemon.log("Failed to start clangd: %v", err)
		os.Exit(1)
	}
	defer daemon.clangdClient.Stop()

	// Setup file watcher
	daemon.fileWatcher, err = NewFileWatcher(projectRoot, daemon.onFilesChanged)
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

func (d *Daemon) setupLogging() error {
	// Truncate log if too large (10MB)
	if err := TruncateLogFile(d.projectRoot, 10*1024*1024); err != nil {
		return err
	}

	logPath := GetLogPath(d.projectRoot)
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}

	d.logFile = logFile
	log.SetOutput(logFile)
	return nil
}

func (d *Daemon) log(format string, args ...interface{}) {
	timestamp := time.Now().Format("2006-01-02 15:04:05.000")
	message := fmt.Sprintf(format, args...)
	fmt.Fprintf(d.logFile, "[%s] %s\n", timestamp, message)
	d.logFile.Sync()
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
		return d.handleLogs()
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

func (d *Daemon) handleLogs() (json.RawMessage, error) {
	logPath := GetLogPath(d.projectRoot)
	
	// Read last 100 lines of log file
	content, err := os.ReadFile(logPath)
	if err != nil {
		return nil, err
	}

	// TODO: Implement tail logic to get last N lines
	return json.Marshal(map[string]string{"logs": string(content)})
}

func (d *Daemon) forwardToClangd(req Request) (json.RawMessage, error) {
	// Extract parameters
	params := req.Params
	
	// Get common parameters
	limit := -1
	if l, ok := params["limit"].(float64); ok {
		limit = int(l)
	}
	
	// Handle each command
	switch req.Method {
	case "search":
		query, ok := params["arguments"].([]interface{})
		if !ok || len(query) == 0 {
			return nil, fmt.Errorf("search requires a query argument")
		}
		queryStr := fmt.Sprintf("%v", query[0])
		
		results, err := commands.Search(d.clangdClient, queryStr, limit)
		if err != nil {
			return nil, err
		}
		return json.Marshal(results)
		
	case "show":
		args, ok := params["arguments"].([]interface{})
		if !ok || len(args) == 0 {
			return nil, fmt.Errorf("show requires a symbol or location argument")
		}
		input := fmt.Sprintf("%v", args[0])
		
		results, err := commands.Show(d.clangdClient, input)
		if err != nil {
			return nil, err
		}
		return json.Marshal(results)
		
	case "view":
		args, ok := params["arguments"].([]interface{})
		if !ok || len(args) == 0 {
			return nil, fmt.Errorf("view requires a symbol or location argument")
		}
		input := fmt.Sprintf("%v", args[0])
		
		result, err := commands.View(d.clangdClient, input)
		if err != nil {
			return nil, err
		}
		return json.Marshal(result)
		
	case "usages":
		args, ok := params["arguments"].([]interface{})
		if !ok || len(args) == 0 {
			return nil, fmt.Errorf("usages requires a symbol or location argument")
		}
		input := fmt.Sprintf("%v", args[0])
		
		results, err := commands.Usages(d.clangdClient, input, limit)
		if err != nil {
			return nil, err
		}
		return json.Marshal(results)
		
	case "hierarchy":
		args, ok := params["arguments"].([]interface{})
		if !ok || len(args) == 0 {
			return nil, fmt.Errorf("hierarchy requires a symbol or location argument")
		}
		input := fmt.Sprintf("%v", args[0])
		
		result, err := commands.Hierarchy(d.clangdClient, input, limit)
		if err != nil {
			return nil, err
		}
		return json.Marshal(result)
		
	case "signature":
		args, ok := params["arguments"].([]interface{})
		if !ok || len(args) == 0 {
			return nil, fmt.Errorf("signature requires a symbol or location argument")
		}
		input := fmt.Sprintf("%v", args[0])
		
		results, err := commands.Signature(d.clangdClient, input)
		if err != nil {
			return nil, err
		}
		return json.Marshal(results)
		
	case "interface":
		args, ok := params["arguments"].([]interface{})
		if !ok || len(args) == 0 {
			return nil, fmt.Errorf("interface requires a symbol or location argument")
		}
		input := fmt.Sprintf("%v", args[0])
		
		result, err := commands.Interface(d.clangdClient, input)
		if err != nil {
			return nil, err
		}
		return json.Marshal(result)
		
	default:
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