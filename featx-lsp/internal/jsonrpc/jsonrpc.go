// Package jsonrpc implements the JSON-RPC 2.0 framing LSP uses over stdio:
// each message is a set of `Name: value` headers, a blank line, then a body of
// exactly Content-Length bytes.
package jsonrpc

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"
)

// Standard JSON-RPC 2.0 error codes, plus the LSP-specific ones this server
// can return.
const (
	CodeParseError     = -32700
	CodeInvalidRequest = -32600
	CodeMethodNotFound = -32601
	CodeInvalidParams  = -32602
	CodeInternalError  = -32603
)

// Error is a JSON-RPC error object.
type Error struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (e *Error) Error() string {
	return fmt.Sprintf("jsonrpc: %d: %s", e.Code, e.Message)
}

// Message is a request, response or notification. An absent ID marks a
// notification, which must not be replied to.
type Message struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *Error          `json:"error,omitempty"`
}

// IsNotification reports whether the message expects no reply.
func (m *Message) IsNotification() bool {
	return len(m.ID) == 0
}

// Conn reads and writes framed messages. Writes are serialised so handlers and
// server-initiated notifications can share one connection.
type Conn struct {
	r *bufio.Reader

	mu sync.Mutex
	w  io.Writer
}

// NewConn wraps a reader and writer, normally os.Stdin and os.Stdout.
func NewConn(r io.Reader, w io.Writer) *Conn {
	return &Conn{r: bufio.NewReader(r), w: w}
}

// Read returns the next message, or io.EOF once the client closes the stream.
func (c *Conn) Read() (*Message, error) {
	length := -1

	for {
		line, err := c.r.ReadString('\n')
		if err != nil {
			return nil, err
		}

		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			break // end of headers
		}

		name, value, found := strings.Cut(line, ":")
		if !found {
			return nil, fmt.Errorf("jsonrpc: malformed header %q", line)
		}

		// Header names are case-insensitive.
		if strings.EqualFold(strings.TrimSpace(name), "Content-Length") {
			length, err = strconv.Atoi(strings.TrimSpace(value))
			if err != nil {
				return nil, fmt.Errorf("jsonrpc: bad Content-Length %q: %w", value, err)
			}
		}
	}

	if length < 0 {
		return nil, fmt.Errorf("jsonrpc: message has no Content-Length header")
	}

	body := make([]byte, length)
	if _, err := io.ReadFull(c.r, body); err != nil {
		return nil, fmt.Errorf("jsonrpc: short body: %w", err)
	}

	var m Message
	if err := json.Unmarshal(body, &m); err != nil {
		return nil, fmt.Errorf("jsonrpc: %w", err)
	}

	return &m, nil
}

// Write frames and sends one message.
func (c *Conn) Write(m *Message) error {
	m.JSONRPC = "2.0"

	body, err := json.Marshal(m)
	if err != nil {
		return fmt.Errorf("jsonrpc: marshal: %w", err)
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if _, err := fmt.Fprintf(c.w, "Content-Length: %d\r\n\r\n", len(body)); err != nil {
		return err
	}

	_, err = c.w.Write(body)
	return err
}

// Reply sends a successful response carrying result.
func (c *Conn) Reply(id json.RawMessage, result any) error {
	raw, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("jsonrpc: marshal result: %w", err)
	}

	return c.Write(&Message{ID: id, Result: raw})
}

// ReplyError sends an error response.
func (c *Conn) ReplyError(id json.RawMessage, code int, format string, args ...any) error {
	return c.Write(&Message{
		ID:    id,
		Error: &Error{Code: code, Message: fmt.Sprintf(format, args...)},
	})
}

// Notify sends a server-initiated notification.
func (c *Conn) Notify(method string, params any) error {
	raw, err := json.Marshal(params)
	if err != nil {
		return fmt.Errorf("jsonrpc: marshal params: %w", err)
	}

	return c.Write(&Message{Method: method, Params: raw})
}
