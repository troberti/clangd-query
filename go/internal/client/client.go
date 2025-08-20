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

	"clangd-query/internal/commands"
	"clangd-query/internal/daemon"
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

// Client handles communication with the daemon
type Client struct {
	conn    net.Conn
	encoder *json.Encoder
	decoder *json.Decoder
	timeout time.Duration
	reqID   int
}

// RPCOptions contains options for RPC calls
type RPCOptions struct {
	Timeout time.Duration // Custom timeout for this call
}

// Request represents a JSON-RPC request
type Request struct {
	ID     int                    `json:"id"`
	Method string                 `json:"method"`
	Params map[string]interface{} `json:"params,omitempty"`
}

// Response represents a JSON-RPC response
type Response struct {
	ID     int             `json:"id"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  *ErrorResponse  `json:"error,omitempty"`
}

// ErrorResponse represents an error in a response
type ErrorResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// StatusInfo represents daemon status
type StatusInfo struct {
	PID           int    `json:"pid"`
	ProjectRoot   string `json:"projectRoot"`
	Uptime        string `json:"uptime"`
	TotalRequests int    `json:"totalRequests"`
	Connections   int    `json:"connections"`
}

// NewClient creates a new client connected to the daemon
func NewClient(conn net.Conn, timeout time.Duration) *Client {
	return &Client{
		conn:    conn,
		encoder: json.NewEncoder(conn),
		decoder: json.NewDecoder(conn),
		timeout: timeout,
		reqID:   1,
	}
}

// CallRPC makes a generic RPC call to the daemon
func (c *Client) CallRPC(method string, params map[string]interface{}, opts *RPCOptions) (json.RawMessage, error) {
	// Use custom timeout if provided
	timeout := c.timeout
	if opts != nil && opts.Timeout > 0 {
		timeout = opts.Timeout
	}

	// Create request
	req := Request{
		ID:     c.reqID,
		Method: method,
		Params: params,
	}
	c.reqID++

	// Send request
	if err := c.encoder.Encode(req); err != nil {
		return nil, fmt.Errorf("failed to send request: %v", err)
	}

	// Set read timeout
	c.conn.SetReadDeadline(time.Now().Add(timeout))

	// Read response
	var resp Response
	if err := c.decoder.Decode(&resp); err != nil {
		if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
			return nil, fmt.Errorf("request timeout after %v", timeout)
		}
		return nil, fmt.Errorf("failed to read response: %v", err)
	}

	// Check for error
	if resp.Error != nil {
		return nil, fmt.Errorf("%s", resp.Error.Message)
	}

	return resp.Result, nil
}

// CallTyped makes an RPC call and unmarshals the result into the provided interface
func (c *Client) CallTyped(method string, params map[string]interface{}, result interface{}) error {
	raw, err := c.CallRPC(method, params, nil)
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, result)
}

// Search searches for symbols
func (c *Client) Search(query string, limit int) ([]commands.SearchResult, error) {
	params := map[string]interface{}{
		"query": query,  // Keep "query" for search since it's a search query, not a symbol
		"limit": limit,
	}

	var results []commands.SearchResult
	err := c.CallTyped("search", params, &results)
	return results, err
}

// Show shows declaration and definition
func (c *Client) Show(symbolOrLocation string) ([]commands.ShowResult, error) {
	params := map[string]interface{}{
		"symbol": symbolOrLocation,
	}

	var results []commands.ShowResult
	err := c.CallTyped("show", params, &results)
	return results, err
}

// View views complete source code
func (c *Client) View(symbolOrLocation string) (*commands.ViewResult, error) {
	params := map[string]interface{}{
		"symbol": symbolOrLocation,
	}

	var result commands.ViewResult
	err := c.CallTyped("view", params, &result)
	return &result, err
}

// Usages finds all usages of a symbol
func (c *Client) Usages(symbolOrLocation string, limit int) ([]commands.UsageResult, error) {
	params := map[string]interface{}{
		"symbol": symbolOrLocation,
		"limit":  limit,
	}

	var results []commands.UsageResult
	err := c.CallTyped("usages", params, &results)
	return results, err
}

// Hierarchy shows type hierarchy
func (c *Client) Hierarchy(symbolOrLocation string, limit int) (*commands.HierarchyResult, error) {
	params := map[string]interface{}{
		"symbol": symbolOrLocation,
		"limit":  limit,
	}

	var result commands.HierarchyResult
	err := c.CallTyped("hierarchy", params, &result)
	return &result, err
}

// Signature shows function signature
func (c *Client) Signature(symbolOrLocation string) ([]commands.SignatureResult, error) {
	params := map[string]interface{}{
		"symbol": symbolOrLocation,
	}

	var results []commands.SignatureResult
	err := c.CallTyped("signature", params, &results)
	return results, err
}

// Interface shows public interface
func (c *Client) Interface(symbolOrLocation string) (*commands.InterfaceResult, error) {
	params := map[string]interface{}{
		"symbol": symbolOrLocation,
	}

	var result commands.InterfaceResult
	err := c.CallTyped("interface", params, &result)
	return &result, err
}

// GetLogs retrieves daemon logs
func (c *Client) GetLogs(level string) (string, error) {
	params := map[string]interface{}{
		"level": level,
	}

	var logsResponse map[string]string
	err := c.CallTyped("logs", params, &logsResponse)
	if err != nil {
		return "", err
	}
	return logsResponse["logs"], nil
}

// GetStatus retrieves daemon status
func (c *Client) GetStatus() (*StatusInfo, error) {
	var status StatusInfo
	err := c.CallTyped("status", map[string]interface{}{}, &status)
	return &status, err
}

// Shutdown initiates daemon shutdown
func (c *Client) Shutdown() error {
	_, err := c.CallRPC("shutdown", map[string]interface{}{}, nil)
	return err
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

	// Create client
	client := NewClient(conn, time.Duration(config.Timeout)*time.Second)

	// Define which commands need a symbol parameter
	symbolCommands := map[string]bool{
		"search":    true,
		"show":      true,
		"view":      true,
		"usages":    true,
		"hierarchy": true,
		"signature": true,
		"interface": true,
	}

	// Extract symbol if needed
	symbol := ""
	if symbolCommands[config.Command] {
		if len(config.Arguments) == 0 {
			return fmt.Errorf("%s requires a symbol argument", config.Command)
		}
		symbol = config.Arguments[0]
	}

	// Execute command
	switch config.Command {
	case "search":
		results, err := client.Search(symbol, config.Limit)
		if err != nil {
			return err
		}
		for _, r := range results {
			fmt.Printf("%s %s:%d:%d %s\n", 
				r.Kind, r.File, r.Line, r.Column, r.Name)
		}

	case "show":
		results, err := client.Show(symbol)
		if err != nil {
			return err
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
		result, err := client.View(symbol)
		if err != nil {
			return err
		}
		fmt.Printf("%s:%d:%d:\n", result.File, result.Line, result.Column)
		fmt.Print(result.Content)
		if !strings.HasSuffix(result.Content, "\n") {
			fmt.Println()
		}

	case "usages":
		results, err := client.Usages(symbol, config.Limit)
		if err != nil {
			return err
		}
		for _, r := range results {
			fmt.Printf("%s:%d:%d: %s\n", 
				r.File, r.Line, r.Column, r.Snippet)
		}

	case "hierarchy":
		result, err := client.Hierarchy(symbol, config.Limit)
		if err != nil {
			return err
		}
		fmt.Print(result.Tree)

	case "signature":
		results, err := client.Signature(symbol)
		if err != nil {
			return err
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
		result, err := client.Interface(symbol)
		if err != nil {
			return err
		}
		fmt.Printf("Public interface of %s:\n\n", result.Name)
		for _, member := range result.Members {
			fmt.Printf("%s\n", member.Signature)
			if member.Documentation != "" {
				fmt.Printf("  %s\n", member.Documentation)
			}
			fmt.Println()
		}

	case "logs":
		// Parse log level from arguments
		logLevel := "info" // default
		for _, arg := range config.Arguments {
			if arg == "--verbose" || arg == "-v" {
				logLevel = "verbose"
			} else if arg == "--error" || arg == "-e" {
				logLevel = "error"
			}
		}
		// Global verbose flag overrides
		if config.Verbose {
			logLevel = "verbose"
		}

		logs, err := client.GetLogs(logLevel)
		if err != nil {
			return err
		}
		fmt.Print(logs)

	case "status":
		status, err := client.GetStatus()
		if err != nil {
			return err
		}
		fmt.Printf("Daemon Status:\n")
		fmt.Printf("  PID: %d\n", status.PID)
		fmt.Printf("  Project: %s\n", status.ProjectRoot)
		fmt.Printf("  Uptime: %s\n", status.Uptime)
		fmt.Printf("  Requests: %d\n", status.TotalRequests)
		fmt.Printf("  Connections: %d\n", status.Connections)

	case "shutdown":
		if err := client.Shutdown(); err != nil {
			return err
		}
		fmt.Println("Daemon shutdown initiated")

	default:
		return fmt.Errorf("unknown command: %s", config.Command)
	}

	return nil
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
	args := []string{"daemon", projectRoot}
	if verbose {
		args = append(args, "--verbose")
	}
	cmd := exec.Command(execPath, args...)
	
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