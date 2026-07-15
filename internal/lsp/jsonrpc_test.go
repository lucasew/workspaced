package lsp

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestConnRoundTrip(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	c := NewConn(strings.NewReader(""), &buf)
	msg := &Message{
		JSONRPC: "2.0",
		ID:      json.RawMessage("1"),
		Method:  "initialize",
		Params:  json.RawMessage(`{"rootUri":"file:///tmp"}`),
	}
	if err := c.WriteMessage(msg); err != nil {
		t.Fatal(err)
	}
	raw := buf.String()
	if !strings.Contains(raw, "Content-Length:") {
		t.Fatalf("header missing: %q", raw)
	}
	rc := NewConn(strings.NewReader(raw), &buf)
	got, err := rc.ReadMessage()
	if err != nil {
		t.Fatal(err)
	}
	if got.Method != "initialize" {
		t.Fatalf("method=%s", got.Method)
	}
	if !got.IsRequest() {
		t.Fatal("expected request")
	}
}

func TestMergeResultsArrays(t *testing.T) {
	t.Parallel()
	out := mergeResults([]json.RawMessage{
		json.RawMessage(`[{"uri":"a"}]`),
		json.RawMessage(`[{"uri":"b"}]`),
		json.RawMessage(`null`),
	})
	var items []map[string]string
	if err := json.Unmarshal(out, &items); err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Fatalf("len=%d %s", len(items), out)
	}
}

func TestMergeResultsFirstNonNull(t *testing.T) {
	t.Parallel()
	out := mergeResults([]json.RawMessage{
		json.RawMessage(`null`),
		json.RawMessage(`{"contents":"hi"}`),
		json.RawMessage(`{"contents":"other"}`),
	})
	if string(out) != `{"contents":"hi"}` {
		t.Fatalf("got %s", out)
	}
}
