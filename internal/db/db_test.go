package db

import (
	"path/filepath"
	"testing"

	"github.com/lucasew/workspaced/internal/types"
	"github.com/lucasew/workspaced/pkg/logging"
)

func TestOpenURLMigratesAndQueries(t *testing.T) {
	ctx := logging.NewWriterContext(t.Output())
	path := filepath.Join(t.TempDir(), "workspaced.db")
	d, err := OpenURL(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := d.Close(); err != nil {
			t.Errorf("close: %v", err)
		}
	})

	ev := types.HistoryEvent{
		Command:   "workspaced home apply",
		Cwd:       "/tmp",
		Timestamp: 1,
		ExitCode:  0,
		Duration:  10,
	}
	if err := d.RecordHistory(ctx, ev); err != nil {
		t.Fatal(err)
	}
	got, err := d.SearchHistory(ctx, "", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Command != ev.Command {
		t.Fatalf("got %#v", got)
	}
}

func TestOpenURLRejectsUnknownScheme(t *testing.T) {
	ctx := logging.NewWriterContext(t.Output())
	_, err := OpenURL(ctx, "postgres://localhost/app")
	if err == nil {
		t.Fatal("expected error")
	}
}
