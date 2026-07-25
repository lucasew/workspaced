package deployer

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/lucasew/workspaced/internal/source"
	"github.com/lucasew/workspaced/pkg/logging"
	"github.com/lucasew/workspaced/pkg/taskgroup"
)

type errReadCloser struct {
	err error
}

func (e errReadCloser) Read([]byte) (int, error) { return 0, e.err }
func (e errReadCloser) Close() error             { return nil }

type failingReaderFile struct {
	source.BasicFile
	openErr error
	readErr error
}

func (f *failingReaderFile) Reader() (io.ReadCloser, error) {
	if f.openErr != nil {
		return nil, f.openErr
	}
	return errReadCloser{err: f.readErr}, nil
}

func TestExecuteRemovesPartialOnCopyError(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	target := filepath.Join(dir, "managed.txt")
	// Pre-existing good file is removed before write; a failed copy must not
	// leave a truncated replacement behind.
	if err := os.WriteFile(target, []byte("good"), 0o644); err != nil {
		t.Fatal(err)
	}

	actions := []Action{{
		Type:   ActionUpdate,
		Target: target,
		Desired: DesiredState{
			File: &failingReaderFile{
				BasicFile: source.BasicFile{
					RelPathStr:    "managed.txt",
					TargetBaseDir: dir,
					FileMode:      0o644,
					Info:          "test:failing-reader",
					FileType:      source.TypeStatic,
				},
				readErr: errors.New("read boom"),
			},
		},
	}}
	state := &State{Files: map[string]ManagedInfo{
		target: {SourceInfo: "test:old"},
	}}

	g, ctx := taskgroup.New(logging.NewWriterContext(t.Output()), taskgroup.DefaultLimits())
	_ = g
	err := NewExecutor().Execute(ctx, actions, state)
	if err == nil {
		t.Fatal("expected copy error")
	}
	if _, statErr := os.Stat(target); !os.IsNotExist(statErr) {
		t.Fatalf("partial file still present after error: stat=%v execute=%v", statErr, err)
	}
	if _, ok := state.Files[target]; !ok {
		t.Fatal("state should not drop managed entry when apply fails")
	}
}

func TestExecuteRemovesEmptyFileOnReaderError(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	target := filepath.Join(dir, "managed.txt")

	actions := []Action{{
		Type:   ActionCreate,
		Target: target,
		Desired: DesiredState{
			File: &failingReaderFile{
				BasicFile: source.BasicFile{
					RelPathStr:    "managed.txt",
					TargetBaseDir: dir,
					FileMode:      0o644,
					Info:          "test:open-fail",
					FileType:      source.TypeStatic,
				},
				openErr: errors.New("open boom"),
			},
		},
	}}
	state := &State{Files: map[string]ManagedInfo{}}

	g, ctx := taskgroup.New(logging.NewWriterContext(t.Output()), taskgroup.DefaultLimits())
	_ = g
	err := NewExecutor().Execute(ctx, actions, state)
	if err == nil {
		t.Fatal("expected reader error")
	}
	if _, statErr := os.Stat(target); !os.IsNotExist(statErr) {
		t.Fatalf("empty file still present after reader error: stat=%v execute=%v", statErr, err)
	}
}

func TestExecuteWritesRegularFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	target := filepath.Join(dir, "managed.txt")
	content := []byte("hello apply\n")

	actions := []Action{{
		Type:   ActionCreate,
		Target: target,
		Desired: DesiredState{
			File: &source.BufferFile{
				BasicFile: source.BasicFile{
					RelPathStr:    "managed.txt",
					TargetBaseDir: dir,
					FileMode:      0o644,
					Info:          "test:buffer",
					FileType:      source.TypeStatic,
				},
				Content: content,
			},
		},
	}}
	state := &State{Files: map[string]ManagedInfo{}}

	g, ctx := taskgroup.New(logging.NewWriterContext(t.Output()), taskgroup.DefaultLimits())
	_ = g
	if err := NewExecutor().Execute(ctx, actions, state); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(content) {
		t.Fatalf("got %q, want %q", got, content)
	}
	if info, ok := state.Files[target]; !ok || info.SourceInfo != "test:buffer" {
		t.Fatalf("state not updated: %+v", state.Files)
	}
}
