package lsp

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lucasew/workspaced/pkg/logging"
)

func writeLSP(t *testing.T, w io.Writer, v any) {
	t.Helper()
	body, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.WriteString(w, "Content-Length: "); err != nil {
		t.Fatal(err)
	}
	if _, err := io.WriteString(w, string(mustJSON(len(body)))); err != nil {
		t.Fatal(err)
	}
	if _, err := io.WriteString(w, "\r\n\r\n"); err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write(body); err != nil {
		t.Fatal(err)
	}
}

func mustJSON(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return b
}

func TestProxyInitializeEmptyConfig(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	serverIn, clientToServer := io.Pipe()    // client writes → server reads
	clientFromServer, serverOut := io.Pipe() // server writes → client reads

	ctx, cancel := context.WithCancel(logging.NewWriterContext(io.Discard))
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- Run(ctx, serverIn, serverOut)
	}()

	writeLSP(t, clientToServer, map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "initialize",
		"params": map[string]any{
			"processId":    nil,
			"rootUri":      pathToURI(root),
			"capabilities": map[string]any{},
		},
	})

	conn := NewConn(clientFromServer, io.Discard)
	msg, err := conn.ReadMessage()
	if err != nil {
		t.Fatal(err)
	}
	if msg.Error != nil {
		t.Fatalf("error: %+v", msg.Error)
	}
	if len(msg.Result) == 0 {
		t.Fatal("empty result")
	}
	var result map[string]any
	if err := json.Unmarshal(msg.Result, &result); err != nil {
		t.Fatal(err)
	}
	if result["capabilities"] == nil {
		t.Fatal("missing capabilities")
	}

	// hover with no backends -> method not found
	writeLSP(t, clientToServer, map[string]any{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "textDocument/hover",
		"params": map[string]any{
			"textDocument": map[string]any{"uri": pathToURI(filepath.Join(root, "x.go"))},
			"position":     map[string]any{"line": 0, "character": 0},
		},
	})
	msg, err = conn.ReadMessage()
	if err != nil {
		t.Fatal(err)
	}
	if msg.Error == nil || msg.Error.Code != CodeMethodNotFound {
		t.Fatalf("want MethodNotFound, got %+v result=%s", msg.Error, msg.Result)
	}

	_ = clientToServer.Close()
	_ = serverOut.Close()
	select {
	case <-errCh:
	case <-time.After(2 * time.Second):
		cancel()
	}
}

func TestProxyMultiRootRejected(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	serverIn, clientToServer := io.Pipe()
	clientFromServer, serverOut := io.Pipe()

	ctx, cancel := context.WithCancel(logging.NewWriterContext(io.Discard))
	defer cancel()
	go func() { _ = Run(ctx, serverIn, serverOut) }()

	writeLSP(t, clientToServer, map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "initialize",
		"params": map[string]any{
			"workspaceFolders": []map[string]any{
				{"uri": pathToURI(root), "name": "a"},
				{"uri": pathToURI(filepath.Join(root, "other")), "name": "b"},
			},
			"capabilities": map[string]any{},
		},
	})
	conn := NewConn(clientFromServer, io.Discard)
	msg, err := conn.ReadMessage()
	if err != nil {
		t.Fatal(err)
	}
	if msg.Error == nil || !strings.Contains(msg.Error.Message, "single workspace") {
		t.Fatalf("want multi-root error, got %+v", msg.Error)
	}
	_ = clientToServer.Close()
	cancel()
}

func TestWriteReadFraming(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	c := NewConn(nil, &buf)
	if err := c.WriteResult(json.RawMessage("1"), map[string]string{"ok": "yes"}); err != nil {
		t.Fatal(err)
	}
	rc := NewConn(bytes.NewReader(buf.Bytes()), io.Discard)
	msg, err := rc.ReadMessage()
	if err != nil {
		t.Fatal(err)
	}
	if string(msg.ID) != "1" {
		t.Fatalf("id=%s", msg.ID)
	}
}
