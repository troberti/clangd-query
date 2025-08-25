package lsp

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Request represents a JSON-RPC 2.0 request message that expects a response.
// Requests always have an ID field that correlates the response back to the request.
type Request struct {
	Jsonrpc string          `json:"jsonrpc"`
	ID      interface{}     `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// Response represents a JSON-RPC 2.0 response message.
// Every response includes the ID from the original request for correlation.
// Either Result or Error will be set, but never both.
type Response struct {
	Jsonrpc string          `json:"jsonrpc"`
	ID      interface{}     `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *Error          `json:"error,omitempty"`
}

// Notification represents a JSON-RPC 2.0 notification message.
// Notifications are fire-and-forget messages that don't expect a response.
// They lack an ID field, which distinguishes them from requests.
type Notification struct {
	Jsonrpc string          `json:"jsonrpc"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// Error represents a JSON-RPC 2.0 error object returned in a response.
// The Code field uses standard JSON-RPC error codes when applicable.
type Error struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

// Standard JSON-RPC 2.0 error codes as defined in the specification.
// These codes indicate protocol-level errors rather than application errors.
const (
	ParseError     = -32700 // Invalid JSON was received
	InvalidRequest = -32600 // JSON is not a valid Request object
	MethodNotFound = -32601 // Method does not exist or is not available
	InvalidParams  = -32602 // Invalid method parameters
	InternalError  = -32603 // Internal JSON-RPC error
)

// Common transport errors that can occur during JSON-RPC communication.
// These are transport-level errors, distinct from JSON-RPC protocol errors.
var (
	ErrConnectionClosed = errors.New("connection closed")
	ErrTimeout          = errors.New("request timeout")
)

// Transport manages synchronous JSON-RPC 2.0 communication over stdin/stdout.
// It implements a simple request-response model where each request blocks until
// its response is received. This design eliminates complexity around concurrent
// request tracking while still supporting asynchronous notifications from the server.
//
// The transport ensures thread-safety by serializing all requests through a mutex,
// making it impossible to have response mismatches or race conditions.
type Transport struct {
	reader *bufio.Reader
	writer io.Writer
	stderr io.Writer

	nextID int64      // Atomic counter for generating unique request IDs
	mu     sync.Mutex // Serializes all requests and protects the closed flag
	closed bool       // Set to true when the connection fails or closes

	handlers   map[string]NotificationHandler // Registered handlers for server notifications
	handlersMu sync.RWMutex                   // Protects the handlers map
}

// NotificationHandler processes incoming notifications from the server.
// Handlers are called asynchronously when notifications arrive.
type NotificationHandler func(params json.RawMessage)

// Creates a new Transport for JSON-RPC communication.
// The stdin parameter is used for reading responses and notifications,
// stdout for writing requests and notifications, and stderr for error logging.
func NewTransport(stdin io.Reader, stdout, stderr io.Writer) *Transport {
	return &Transport{
		reader:   bufio.NewReader(stdin),
		writer:   stdout,
		stderr:   stderr,
		handlers: make(map[string]NotificationHandler),
	}
}

// Registers a handler for a specific notification method.
// When a notification with the given method name arrives from the server,
// the handler will be called asynchronously with the notification parameters.
// Only one handler can be registered per method; subsequent registrations
// will replace the previous handler.
func (t *Transport) RegisterNotificationHandler(method string, handler NotificationHandler) {
	t.handlersMu.Lock()
	defer t.handlersMu.Unlock()
	t.handlers[method] = handler
}

// Sends a JSON-RPC request and blocks until the response is received.
// This method is thread-safe and ensures that only one request is in flight at a time.
// The method automatically generates a unique ID for request correlation and handles
// timeout (30 seconds) to prevent indefinite blocking. Any notifications received
// while waiting for the response are dispatched to their registered handlers.
func (t *Transport) SendRequest(method string, params interface{}) (json.RawMessage, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.closed {
		return nil, ErrConnectionClosed
	}

	// Generate unique string ID to avoid JSON number type ambiguity
	id := strconv.FormatInt(atomic.AddInt64(&t.nextID, 1), 10)

	paramsJSON, err := json.Marshal(params)
	if err != nil {
		return nil, fmt.Errorf("Error encoding request params: %w", err)
	}

	req := Request{
		Jsonrpc: "2.0",
		ID:      id,
		Method:  method,
		Params:  paramsJSON,
	}

	// Write the request to the output stream
	if err := t.writeMessage(req); err != nil {
		t.closed = true
		return nil, fmt.Errorf("Error writing request: %w", err)
	}

	// Read the response in a goroutine to implement timeout
	type result struct {
		resp *Response
		err  error
	}
	done := make(chan result, 1)

	go func() {
		resp, err := t.readResponse(id)
		select {
		case done <- result{resp, err}:
		default:
			// Timeout already occurred, discard result
		}
	}()

	// Block until we get a response or timeout
	select {
	case r := <-done:
		if r.err != nil {
			if errors.Is(r.err, io.EOF) || errors.Is(r.err, io.ErrUnexpectedEOF) {
				t.closed = true
				return nil, ErrConnectionClosed
			}
			return nil, r.err
		}

		if r.resp.Error != nil {
			return nil, fmt.Errorf("RPC error %d: %s", r.resp.Error.Code, r.resp.Error.Message)
		}

		return r.resp.Result, nil

	case <-time.After(30 * time.Second):
		t.closed = true
		return nil, ErrTimeout
	}
}

// Sends a notification to the server without expecting a response.
// Notifications are fire-and-forget messages used for events like
// file opened/closed notifications. This method returns immediately
// after writing the notification to the output stream.
func (t *Transport) SendNotification(method string, params interface{}) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.closed {
		return ErrConnectionClosed
	}

	paramsJSON, err := json.Marshal(params)
	if err != nil {
		return fmt.Errorf("Error encoding request params: %w", err)
	}

	notif := Notification{
		Jsonrpc: "2.0",
		Method:  method,
		Params:  paramsJSON,
	}

	if err := t.writeMessage(notif); err != nil {
		t.closed = true
		return fmt.Errorf("Error writing notification: %w", err)
	}

	return nil
}

// Placeholder for notification reader initialization.
// In this synchronous implementation, notifications are processed inline
// while waiting for responses, avoiding the need for a separate reader goroutine.
// This design prevents reader conflicts and simplifies the implementation.
func (t *Transport) Start() {
	// Notifications are handled during readResponse, not in a separate goroutine
}

// Reads messages from the input stream until it finds a response with the expected ID.
// Any notifications encountered while waiting are dispatched to their handlers.
// This approach ensures we never miss notifications even while waiting for a response.
func (t *Transport) readResponse(expectedID string) (*Response, error) {
	for {
		msg, err := t.readMessage()
		if err != nil {
			return nil, err
		}

		// Found the response we're waiting for
		if id, ok := msg["id"].(string); ok && id == expectedID {
			var resp Response
			data, _ := json.Marshal(msg)
			if err := json.Unmarshal(data, &resp); err != nil {
				return nil, fmt.Errorf("Error decoding response: %w", err)
			}
			return &resp, nil
		}

		// Process any notifications that arrive while waiting
		if msg["id"] == nil && msg["method"] != nil {
			var notif Notification
			if data, _ := json.Marshal(msg); data != nil {
				if err := json.Unmarshal(data, &notif); err == nil {
					go func() {
						t.handleNotification(&notif)
					}()
				}
			}
		}
		// Ignore responses for other IDs (shouldn't happen in synchronous mode)
	}
}

// Reads a single JSON-RPC message from the input stream.
// Messages use HTTP-style headers with Content-Length to frame the JSON payload.
// This is the standard format used by the Language Server Protocol.
func (t *Transport) readMessage() (map[string]interface{}, error) {
	// Parse HTTP-style headers
	var contentLength int
	for {
		line, err := t.reader.ReadString('\n')
		if err != nil {
			return nil, err
		}

		line = strings.TrimSpace(line)
		if line == "" {
			break // Empty line indicates end of headers
		}

		if strings.HasPrefix(line, "Content-Length: ") {
			lengthStr := strings.TrimPrefix(line, "Content-Length: ")
			length, err := strconv.Atoi(strings.TrimSpace(lengthStr))
			if err != nil {
				return nil, fmt.Errorf("invalid Content-Length: %w", err)
			}
			// Sanity check: messages shouldn't be larger than 10MB
			if length < 0 || length > 10*1024*1024 {
				return nil, fmt.Errorf("invalid Content-Length %d: must be between 0 and 10MB", length)
			}
			contentLength = length
		}
	}

	if contentLength == 0 {
		return nil, errors.New("missing Content-Length header")
	}

	// Read the JSON payload based on Content-Length
	content := make([]byte, contentLength)
	n, err := io.ReadFull(t.reader, content)
	if err != nil {
		return nil, err
	}
	if n != contentLength {
		return nil, fmt.Errorf("content length mismatch: expected %d, got %d", contentLength, n)
	}

	// Unmarshal into generic map to determine message type
	var msg map[string]interface{}
	if err := json.Unmarshal(content, &msg); err != nil {
		return nil, fmt.Errorf("parse message: %w", err)
	}

	return msg, nil
}

// Dispatches a notification to its registered handler if one exists.
// Handlers are called asynchronously to avoid blocking message processing.
// If no handler is registered for the notification method, it's silently ignored.
func (t *Transport) handleNotification(notif *Notification) {
	t.handlersMu.RLock()
	handler, ok := t.handlers[notif.Method]
	t.handlersMu.RUnlock()

	if ok {
		handler(notif.Params)
	}
}

// Writes a JSON-RPC message to the output stream with proper framing.
// The message is preceded by HTTP-style headers including Content-Length.
// This method assumes the caller holds the transport mutex.
func (t *Transport) writeMessage(msg interface{}) error {
	content, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	header := fmt.Sprintf("Content-Length: %d\r\n\r\n", len(content))

	if _, err := t.writer.Write([]byte(header)); err != nil {
		return err
	}

	if _, err := t.writer.Write(content); err != nil {
		return err
	}

	return nil
}
