package lsp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
)

// JSON-RPC 2.0 message types

type Request struct {
	Jsonrpc string          `json:"jsonrpc"`
	ID      interface{}     `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type Response struct {
	Jsonrpc string          `json:"jsonrpc"`
	ID      interface{}     `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *Error          `json:"error,omitempty"`
}

type Notification struct {
	Jsonrpc string          `json:"jsonrpc"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type Error struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

// Standard JSON-RPC error codes
const (
	ParseError     = -32700
	InvalidRequest = -32600
	MethodNotFound = -32601
	InvalidParams  = -32602
	InternalError  = -32603
)

// Transport handles reading and writing JSON-RPC messages over stdio
type Transport struct {
	stdin  io.Reader
	stdout io.Writer
	stderr io.Writer

	nextID  int64
	pending map[interface{}]chan *Response
	mu      sync.Mutex

	handlers      map[string]NotificationHandler
	handlersMu    sync.RWMutex
}

type NotificationHandler func(params json.RawMessage)

func NewTransport(stdin io.Reader, stdout, stderr io.Writer) *Transport {
	return &Transport{
		stdin:    stdin,
		stdout:   stdout,
		stderr:   stderr,
		pending:  make(map[interface{}]chan *Response),
		handlers: make(map[string]NotificationHandler),
	}
}

// RegisterNotificationHandler registers a handler for notifications
func (t *Transport) RegisterNotificationHandler(method string, handler NotificationHandler) {
	t.handlersMu.Lock()
	defer t.handlersMu.Unlock()
	t.handlers[method] = handler
}

// SendRequest sends a request and waits for the response
func (t *Transport) SendRequest(method string, params interface{}) (json.RawMessage, error) {
	id := atomic.AddInt64(&t.nextID, 1)
	
	paramsJSON, err := json.Marshal(params)
	if err != nil {
		return nil, err
	}

	req := Request{
		Jsonrpc: "2.0",
		ID:      id,
		Method:  method,
		Params:  paramsJSON,
	}

	// Create response channel before sending
	// Use float64 as key since JSON numbers are parsed as float64
	respChan := make(chan *Response, 1)
	t.mu.Lock()
	t.pending[float64(id)] = respChan
	t.mu.Unlock()

	// Send request
	if err := t.writeMessage(req); err != nil {
		t.mu.Lock()
		delete(t.pending, float64(id))
		t.mu.Unlock()
		return nil, err
	}

	// Wait for response
	resp := <-respChan
	
	if resp.Error != nil {
		return nil, fmt.Errorf("RPC error %d: %s", resp.Error.Code, resp.Error.Message)
	}

	return resp.Result, nil
}

// SendNotification sends a notification (no response expected)
func (t *Transport) SendNotification(method string, params interface{}) error {
	paramsJSON, err := json.Marshal(params)
	if err != nil {
		return err
	}

	notif := Notification{
		Jsonrpc: "2.0",
		Method:  method,
		Params:  paramsJSON,
	}

	return t.writeMessage(notif)
}

// Start begins reading messages from stdin
func (t *Transport) Start() {
	go t.readLoop()
}

func (t *Transport) readLoop() {
	reader := bufio.NewReader(t.stdin)
	
	for {
		// Read headers
		var contentLength int
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				if err != io.EOF {
					fmt.Fprintf(t.stderr, "Error reading header: %v\n", err)
				}
				return
			}
			
			line = strings.TrimSpace(line)
			
			if line == "" {
				// Empty line marks end of headers
				break
			}
			
			if strings.HasPrefix(line, "Content-Length: ") {
				lengthStr := strings.TrimPrefix(line, "Content-Length: ")
				length, err := strconv.Atoi(strings.TrimSpace(lengthStr))
				if err != nil {
					fmt.Fprintf(t.stderr, "Invalid Content-Length: %v\n", err)
					continue
				}
				contentLength = length
			}
			// Ignore other headers like Content-Type
		}
		
		if contentLength == 0 {
			continue
		}
		
		// Read content
		content := make([]byte, contentLength)
		n, err := io.ReadFull(reader, content)
		if err != nil {
			fmt.Fprintf(t.stderr, "Failed to read message content: %v\n", err)
			return
		}
		if n != contentLength {
			fmt.Fprintf(t.stderr, "Content length mismatch: expected %d, got %d\n", contentLength, n)
			continue
		}
		
		// Parse and handle message
		t.handleMessage(content)
	}
}

func (t *Transport) handleMessage(content []byte) {
	// Try to parse as different message types
	var msg map[string]interface{}
	if err := json.Unmarshal(content, &msg); err != nil {
		fmt.Fprintf(t.stderr, "Failed to parse message: %v\n", err)
		return
	}

	// Check if it's a response (has id but no method)
	if id, hasID := msg["id"]; hasID {
		if _, hasMethod := msg["method"]; !hasMethod {
			// It's a response
			var resp Response
			if err := json.Unmarshal(content, &resp); err != nil {
				fmt.Fprintf(t.stderr, "Failed to parse response: %v\n", err)
				return
			}
			
			t.mu.Lock()
			if ch, ok := t.pending[id]; ok {
				ch <- &resp
				delete(t.pending, id)
			}
			t.mu.Unlock()
			return
		}
	}

	// Check if it's a notification (no id)
	if _, hasID := msg["id"]; !hasID {
		if method, hasMethod := msg["method"].(string); hasMethod {
			var notif Notification
			if err := json.Unmarshal(content, &notif); err != nil {
				fmt.Fprintf(t.stderr, "Failed to parse notification: %v\n", err)
				return
			}

			// Handle notification
			t.handlersMu.RLock()
			handler, ok := t.handlers[method]
			t.handlersMu.RUnlock()

			if ok {
				go handler(notif.Params)
			}
		}
	}

	// TODO: Handle requests from server (we don't expect any from clangd)
}

func (t *Transport) writeMessage(msg interface{}) error {
	content, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	header := fmt.Sprintf("Content-Length: %d\r\n\r\n", len(content))
	
	t.mu.Lock()
	defer t.mu.Unlock()

	if _, err := t.stdout.Write([]byte(header)); err != nil {
		return err
	}

	if _, err := t.stdout.Write(content); err != nil {
		return err
	}

	return nil
}