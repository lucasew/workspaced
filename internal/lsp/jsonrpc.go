package lsp

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"
)

// Message is a JSON-RPC 2.0 message (request, response, or notification).
type Message struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *RespError      `json:"error,omitempty"`
}

// RespError is a JSON-RPC error object.
type RespError struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

// Error codes (JSON-RPC + LSP).
const (
	CodeParseError     = -32700
	CodeInvalidRequest = -32600
	CodeMethodNotFound = -32601
	CodeInvalidParams  = -32602
	CodeInternalError  = -32603
	CodeRequestFailed  = -32803 // LSP
)

// IsRequest reports whether the message expects a response.
func (m *Message) IsRequest() bool {
	return m.Method != "" && len(m.ID) > 0 && string(m.ID) != "null"
}

// IsNotification reports a method call with no id.
func (m *Message) IsNotification() bool {
	return m.Method != "" && (len(m.ID) == 0 || string(m.ID) == "null")
}

// IsResponse reports a result/error reply.
func (m *Message) IsResponse() bool {
	return m.Method == "" && len(m.ID) > 0
}

// Conn is a Content-Length framed JSON-RPC stream pair.
type Conn struct {
	in  *bufio.Reader
	out io.Writer
	mu  sync.Mutex // serializes writes
}

// NewConn wraps reader/writer for LSP framing.
func NewConn(r io.Reader, w io.Writer) *Conn {
	return &Conn{in: bufio.NewReader(r), out: w}
}

// ReadMessage reads one framed message.
func (c *Conn) ReadMessage() (*Message, error) {
	contentLen := -1
	for {
		line, err := c.in.ReadString('\n')
		if err != nil {
			return nil, err
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			break
		}
		const prefix = "Content-Length:"
		if strings.HasPrefix(strings.ToLower(line), strings.ToLower(prefix)) {
			v := strings.TrimSpace(line[len(prefix):])
			n, err := strconv.Atoi(v)
			if err != nil {
				return nil, fmt.Errorf("content-length: %w", err)
			}
			contentLen = n
		}
	}
	if contentLen < 0 {
		return nil, fmt.Errorf("missing Content-Length")
	}
	body := make([]byte, contentLen)
	if _, err := io.ReadFull(c.in, body); err != nil {
		return nil, err
	}
	var msg Message
	if err := json.Unmarshal(body, &msg); err != nil {
		return nil, fmt.Errorf("decode jsonrpc: %w", err)
	}
	if msg.JSONRPC == "" {
		msg.JSONRPC = "2.0"
	}
	return &msg, nil
}

// WriteMessage writes one framed message.
func (c *Conn) WriteMessage(msg *Message) error {
	if msg.JSONRPC == "" {
		msg.JSONRPC = "2.0"
	}
	body, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	var hdr bytes.Buffer
	fmt.Fprintf(&hdr, "Content-Length: %d\r\n\r\n", len(body))
	if _, err := c.out.Write(hdr.Bytes()); err != nil {
		return err
	}
	_, err = c.out.Write(body)
	return err
}

// WriteError responds to a request with an error.
func (c *Conn) WriteError(id json.RawMessage, code int, message string) error {
	return c.WriteMessage(&Message{
		JSONRPC: "2.0",
		ID:      id,
		Error:   &RespError{Code: code, Message: message},
	})
}

// WriteResult responds to a request with a result payload.
func (c *Conn) WriteResult(id json.RawMessage, result any) error {
	raw, err := json.Marshal(result)
	if err != nil {
		return err
	}
	return c.WriteMessage(&Message{
		JSONRPC: "2.0",
		ID:      id,
		Result:  raw,
	})
}

// WriteNotification sends a method with params and no id.
func (c *Conn) WriteNotification(method string, params any) error {
	var raw json.RawMessage
	if params != nil {
		b, err := json.Marshal(params)
		if err != nil {
			return err
		}
		raw = b
	}
	return c.WriteMessage(&Message{
		JSONRPC: "2.0",
		Method:  method,
		Params:  raw,
	})
}
