package wm

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	dapi "github.com/lucasew/workspaced/pkg/api"
	"github.com/lucasew/workspaced/pkg/logging"
)

func TestJSONViaCmd(t *testing.T) {
	t.Parallel()
	ctx := logging.NewWriterContext(t.Output())

	path := filepath.Join(t.TempDir(), "out.json")
	if err := os.WriteFile(path, []byte(`{"name":"HDMI-A-1","x":12}`), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := JSONViaCmd[struct {
		Name string `json:"name"`
		X    int    `json:"x"`
	}](ctx, "cat", path)
	if err != nil {
		t.Fatalf("JSONViaCmd: %v", err)
	}
	if got.Name != "HDMI-A-1" || got.X != 12 {
		t.Fatalf("got %+v", got)
	}
}

func TestJSONViaCmdFailed(t *testing.T) {
	t.Parallel()
	ctx := logging.NewWriterContext(t.Output())
	_, err := JSONViaCmd[struct{}](ctx, "false")
	if err == nil {
		t.Fatal("expected command failure")
	}
	if !errors.Is(err, dapi.ErrIPC) {
		t.Fatalf("err=%v want ErrIPC", err)
	}
	var ee *exec.ExitError
	if !errors.As(err, &ee) {
		t.Fatalf("err=%v want ExitError", err)
	}
}

func TestJSONViaCmdBadJSON(t *testing.T) {
	t.Parallel()
	ctx := logging.NewWriterContext(t.Output())
	path := filepath.Join(t.TempDir(), "not.json")
	if err := os.WriteFile(path, []byte("not-json"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := JSONViaCmd[struct{}](ctx, "cat", path)
	if err == nil {
		t.Fatal("expected decode failure")
	}
	if !errors.Is(err, dapi.ErrIPC) {
		t.Fatalf("err=%v want ErrIPC", err)
	}
	var se *json.SyntaxError
	if !errors.As(err, &se) {
		t.Fatalf("err=%v want SyntaxError", err)
	}
}
