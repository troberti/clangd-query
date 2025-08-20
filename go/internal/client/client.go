package client

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"

	"github.com/firi/clangd-query/internal/commands"
	"github.com/firi/clangd-query/internal/daemon"
)

// Config contains client configuration
type Config struct {
	Command     string
	Arguments   []string
	Limit       int
	Verbose     bool
	Timeout     int
	ProjectRoot string
}

// Request represents a request to the daemon
type Request struct {
	ID     int                    `json:"id"`
	Method string                 `json:"method"`
	Params map[string]interface{} `json:"params,omitempty"`
}

// Response represents a response from the daemon
type Response struct {
	ID     int             `json:"id"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  *ErrorResponse  `json:"error,omitempty"`
}

// ErrorResponse represents an error response
type ErrorResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// Run executes the client with the given configuration
func Run(config *Config) error {
	// Get project root from config
	projectRoot := config.ProjectRoot
	if projectRoot == "" {
		return fmt.Errorf("project root not set")
	}

	// Check if daemon is running
	lockInfo, err := daemon.ReadLockFile(projectRoot)
	if err != nil {
		return fmt.Errorf("failed to read lock file: %v", err)
	}

	needStart := false
	if lockInfo == nil {
		needStart = true
	} else if !daemon.IsProcessAlive(lockInfo.PID) {
		needStart = true
		daemon.RemoveLockFile(projectRoot)
		daemon.CleanupSocket(lockInfo.SocketPath)
	} else if daemon.IsDaemonStale(lockInfo) {
		// Stop old daemon
		if config.Verbose {
			fmt.Fprintf(os.Stderr, "Stopping stale daemon (PID %d)...\n", lockInfo.PID)
		}
		syscall.Kill(lockInfo.PID, syscall.SIGTERM)
		time.Sleep(500 * time.Millisecond)
		needStart = true
	}

	if needStart {
		if err := startDaemon(projectRoot, config.Verbose); err != nil {
			return fmt.Errorf("failed to start daemon: %v", err)
		}
		
		// Re-read lock file
		lockInfo, err = daemon.ReadLockFile(projectRoot)
		if err != nil || lockInfo == nil {
			return fmt.Errorf("daemon started but lock file not found")
		}
	}

	// Connect to daemon
	conn, err := net.Dial("unix", lockInfo.SocketPath)
	if err != nil {
		return fmt.Errorf("failed to connect to daemon: %v", err)
	}
	defer conn.Close()

	// Handle special commands that don't go through the command system
	switch config.Command {
	case "logs", "status", "shutdown":
		return handleSpecialCommand(conn, config)
	}

	// Execute command through the command system
	return executeCommand(conn, config)
}

func startDaemon(projectRoot string, verbose bool) error {
	if verbose {
		fmt.Fprintf(os.Stderr, "Starting daemon...\n")
	}

	// Get current executable path
	execPath, err := os.Executable()
	if err != nil {
		return err
	}

	// Start daemon as background process
	cmd := exec.Command(execPath, "daemon", projectRoot)
	
	// Detach from current process group
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}

	// Redirect output to daemon log
	logPath := daemon.GetLogPath(projectRoot)
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer logFile.Close()

	cmd.Stdout = logFile
	cmd.Stderr = logFile

	if err := cmd.Start(); err != nil {
		return err
	}

	// Don't wait for it - let it run in background
	go cmd.Wait()

	// Wait for daemon to be ready
	socketPath := daemon.GetSocketPath(projectRoot)
	for i := 0; i < 50; i++ { // 5 seconds timeout
		if _, err := os.Stat(socketPath); err == nil {
			// Socket exists, try to connect
			if conn, err := net.Dial("unix", socketPath); err == nil {
				conn.Close()
				return nil
			}
		}
		time.Sleep(100 * time.Millisecond)
	}

	return fmt.Errorf("daemon failed to start within timeout")
}

func handleSpecialCommand(conn net.Conn, config *Config) error {
	// Send request
	req := Request{
		ID:     1,
		Method: config.Command,
	}

	encoder := json.NewEncoder(conn)
	if err := encoder.Encode(req); err != nil {
		return fmt.Errorf("failed to send request: %v", err)
	}

	// Read response
	decoder := json.NewDecoder(conn)
	var resp Response
	if err := decoder.Decode(&resp); err != nil {
		return fmt.Errorf("failed to read response: %v", err)
	}

	if resp.Error != nil {
		return fmt.Errorf("%s", resp.Error.Message)
	}

	// Handle based on command
	switch config.Command {
	case "logs":
		var result map[string]string
		if err := json.Unmarshal(resp.Result, &result); err != nil {
			return err
		}
		fmt.Print(result["logs"])

	case "status":
		var status map[string]interface{}
		if err := json.Unmarshal(resp.Result, &status); err != nil {
			return err
		}
		fmt.Printf("Daemon Status:\n")
		fmt.Printf("  PID: %v\n", status["pid"])
		fmt.Printf("  Project: %v\n", status["projectRoot"])
		fmt.Printf("  Uptime: %v\n", status["uptime"])
		fmt.Printf("  Requests: %v\n", status["totalRequests"])
		fmt.Printf("  Connections: %v\n", status["connections"])

	case "shutdown":
		fmt.Println("Daemon shutdown initiated")
	}

	return nil
}

func executeCommand(conn net.Conn, config *Config) error {
	// Prepare command parameters
	params := make(map[string]interface{})
	params["command"] = config.Command
	params["arguments"] = config.Arguments
	params["limit"] = config.Limit
	params["timeout"] = config.Timeout
	params["verbose"] = config.Verbose
	params["projectRoot"] = config.ProjectRoot

	// Send request
	req := Request{
		ID:     1,
		Method: config.Command,
		Params: params,
	}

	encoder := json.NewEncoder(conn)
	if err := encoder.Encode(req); err != nil {
		return fmt.Errorf("failed to send request: %v", err)
	}

	// Set read timeout based on command timeout
	conn.SetReadDeadline(time.Now().Add(time.Duration(config.Timeout) * time.Second))

	// Read response
	decoder := json.NewDecoder(conn)
	var resp Response
	if err := decoder.Decode(&resp); err != nil {
		if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
			return fmt.Errorf("request timeout after %d seconds", config.Timeout)
		}
		return fmt.Errorf("failed to read response: %v", err)
	}

	if resp.Error != nil {
		return fmt.Errorf("%s", resp.Error.Message)
	}

	// Parse and display result based on command
	return displayResult(config.Command, resp.Result)
}

func displayResult(command string, result json.RawMessage) error {
	// The result format depends on the command
	// For now, just print the raw JSON
	// This will be replaced with proper formatting when we implement the commands
	
	switch command {
	case "search":
		var results []commands.SearchResult
		if err := json.Unmarshal(result, &results); err != nil {
			// Fallback to raw output
			fmt.Println(string(result))
			return nil
		}
		for _, r := range results {
			fmt.Printf("%s %s:%d:%d %s\n", 
				r.Kind, r.File, r.Line, r.Column, r.Name)
		}

	case "show":
		var results []commands.ShowResult
		if err := json.Unmarshal(result, &results); err != nil {
			fmt.Println(string(result))
			return nil
		}
		for i, r := range results {
			if i > 0 {
				fmt.Println("\n---")
			}
			fmt.Printf("%s:%d:%d:\n", r.File, r.Line, r.Column)
			fmt.Print(r.Content)
			if !strings.HasSuffix(r.Content, "\n") {
				fmt.Println()
			}
		}

	case "view":
		var viewResult commands.ViewResult
		if err := json.Unmarshal(result, &viewResult); err != nil {
			fmt.Println(string(result))
			return nil
		}
		fmt.Printf("%s:%d:%d:\n", viewResult.File, viewResult.Line, viewResult.Column)
		fmt.Print(viewResult.Content)
		if !strings.HasSuffix(viewResult.Content, "\n") {
			fmt.Println()
		}

	case "usages":
		var results []commands.UsageResult
		if err := json.Unmarshal(result, &results); err != nil {
			fmt.Println(string(result))
			return nil
		}
		for _, r := range results {
			fmt.Printf("%s:%d:%d: %s\n", 
				r.File, r.Line, r.Column, r.Snippet)
		}

	case "hierarchy":
		var hierarchyResult commands.HierarchyResult
		if err := json.Unmarshal(result, &hierarchyResult); err != nil {
			fmt.Println(string(result))
			return nil
		}
		fmt.Print(hierarchyResult.Tree)

	case "signature":
		var results []commands.SignatureResult
		if err := json.Unmarshal(result, &results); err != nil {
			fmt.Println(string(result))
			return nil
		}
		for i, r := range results {
			if i > 0 {
				fmt.Println()
			}
			fmt.Println(r.Signature)
			if r.Documentation != "" {
				fmt.Println(r.Documentation)
			}
		}

	case "interface":
		var interfaceResult commands.InterfaceResult
		if err := json.Unmarshal(result, &interfaceResult); err != nil {
			fmt.Println(string(result))
			return nil
		}
		fmt.Printf("Public interface of %s:\n\n", interfaceResult.Name)
		for _, member := range interfaceResult.Members {
			fmt.Printf("%s\n", member.Signature)
			if member.Documentation != "" {
				fmt.Printf("  %s\n", member.Documentation)
			}
			fmt.Println()
		}

	default:
		// Unknown command, just print raw result
		fmt.Println(string(result))
	}

	return nil
}