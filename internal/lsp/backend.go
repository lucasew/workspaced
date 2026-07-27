package lsp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"sync"
	"sync/atomic"

	"github.com/lucasew/workspaced/internal/tool"
	execdriver "github.com/lucasew/workspaced/pkg/driver/exec"
	"github.com/lucasew/workspaced/pkg/logging"
)

// ErrEmptyCmd is returned when an LSP server config has no cmd argv.
var ErrEmptyCmd = errors.New("empty cmd")

// Backend is one running language server process.
type Backend struct {
	ID       string
	ServerID string
	conn     *Conn
	cmd      *exec.Cmd
	cancel   context.CancelFunc

	nextID atomic.Int64

	mu      sync.Mutex
	pending map[string]chan *Message // id string -> response

	onNotification func(serverID string, msg *Message)

	stdin  io.WriteCloser
	stdout io.ReadCloser
}

// StartBackend ensures tools, spawns the server, and runs a read loop.
func StartBackend(ctx context.Context, root string, serverID string, srv Server, onNotification func(serverID string, msg *Message)) (*Backend, error) {
	logger := logging.GetLogger(ctx)
	if len(srv.Cmd) == 0 {
		return nil, fmt.Errorf("%w: server %q", ErrEmptyCmd, serverID)
	}

	argv, envExtra, err := resolveServerCmd(ctx, root, srv)
	if err != nil {
		return nil, fmt.Errorf("server %q: %w", serverID, err)
	}

	runCtx, cancel := context.WithCancel(ctx)
	cmd, err := execdriver.Run(runCtx, argv[0], argv[1:]...)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("server %q: exec: %w", serverID, err)
	}
	cmd.Dir = root
	if len(envExtra) > 0 {
		cmd.Env = append(os.Environ(), envExtra...)
	}
	// Language servers log to stderr; keep visible on our stderr.
	cmd.Stderr = os.Stderr

	stdin, err := cmd.StdinPipe()
	if err != nil {
		cancel()
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		logging.Close(ctx, stdin)
		return nil, err
	}

	if err := cmd.Start(); err != nil {
		cancel()
		return nil, fmt.Errorf("server %q: start: %w", serverID, err)
	}
	logger.Info("lsp backend started", "server", serverID, "cmd", argv, "pid", cmd.Process.Pid)

	b := &Backend{
		ID:             serverID,
		ServerID:       serverID,
		conn:           NewConn(stdout, stdin),
		cmd:            cmd,
		cancel:         cancel,
		pending:        map[string]chan *Message{},
		onNotification: onNotification,
		stdin:          stdin,
		stdout:         stdout,
	}
	go b.readLoop(ctx)
	go func() {
		err := cmd.Wait()
		logger.Info("lsp backend exited", "server", serverID, "error", err)
		b.failPending(fmt.Errorf("server %q exited: %w", serverID, err))
	}()
	return b, nil
}

func resolveServerCmd(ctx context.Context, root string, srv Server) (argv []string, envExtra []string, err error) {
	return tool.ResolveNeedsCmd(ctx, root, srv.Cmd, srv.Needs)
}

func (b *Backend) readLoop(ctx context.Context) {
	logger := logging.GetLogger(ctx)
	for {
		msg, err := b.conn.ReadMessage()
		if err != nil {
			if err != io.EOF && !errors.Is(err, fs.ErrClosed) && !errors.Is(err, io.ErrClosedPipe) {
				logger.Debug("backend read ended", "server", b.ServerID, "error", err)
			}
			b.failPending(err)
			return
		}
		if msg.IsResponse() {
			id := string(msg.ID)
			b.mu.Lock()
			ch, ok := b.pending[id]
			if ok {
				delete(b.pending, id)
			}
			b.mu.Unlock()
			if ok {
				ch <- msg
			}
			continue
		}
		// Notifications and server->client requests: forward upward.
		if b.onNotification != nil {
			b.onNotification(b.ServerID, msg)
		}
	}
}

func (b *Backend) failPending(err error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for id, ch := range b.pending {
		ch <- &Message{
			JSONRPC: "2.0",
			ID:      json.RawMessage(id),
			Error:   &RespError{Code: CodeInternalError, Message: err.Error()},
		}
		delete(b.pending, id)
	}
}

// Notify sends a notification (no response).
func (b *Backend) Notify(method string, params any) error {
	return b.conn.WriteNotification(method, params)
}

// Request sends a request and waits for the response (caller applies timeout via ctx).
func (b *Backend) Request(ctx context.Context, method string, params any) (*Message, error) {
	idNum := b.nextID.Add(1)
	idRaw, marshalErr := json.Marshal(idNum)
	if marshalErr != nil {
		return nil, marshalErr
	}

	var raw json.RawMessage
	if params != nil {
		body, err := json.Marshal(params)
		if err != nil {
			return nil, err
		}
		raw = body
	}

	ch := make(chan *Message, 1)
	idKey := string(idRaw)
	b.mu.Lock()
	b.pending[idKey] = ch
	b.mu.Unlock()

	if err := b.conn.WriteMessage(&Message{
		JSONRPC: "2.0",
		ID:      idRaw,
		Method:  method,
		Params:  raw,
	}); err != nil {
		b.mu.Lock()
		delete(b.pending, idKey)
		b.mu.Unlock()
		return nil, err
	}

	select {
	case <-ctx.Done():
		b.mu.Lock()
		delete(b.pending, idKey)
		b.mu.Unlock()
		return nil, ctx.Err()
	case msg := <-ch:
		return msg, nil
	}
}

// Close stops the backend process.
func (b *Backend) Close() {
	if b.cancel != nil {
		b.cancel()
	}
	if b.stdin != nil {
		_ = b.stdin.Close()
	}
	if b.cmd != nil && b.cmd.Process != nil {
		_ = b.cmd.Process.Kill()
	}
}
