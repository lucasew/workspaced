package backup_test

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lucasew/workspaced/internal/backup"
	"github.com/lucasew/workspaced/pkg/logging"
)

func TestArchiveAction_RunValidation(t *testing.T) {
	t.Parallel()

	ctx := logging.NewWriterContext(t.Output())

	tests := []struct {
		name    string
		action  backup.ArchiveAction
		wantErr error
	}{
		{
			name:    "missing input and output",
			action:  backup.ArchiveAction{},
			wantErr: backup.ErrArchiveNeedsInputAndOutput,
		},
		{
			name: "unsupported format",
			action: backup.ArchiveAction{
				InputDir: "/tmp/in",
				Output:   "/tmp/out.tar",
				Format:   "zip",
			},
			wantErr: backup.ErrUnsupportedArchiveFormat,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := tt.action.Run(ctx, nil)
			if err == nil {
				t.Fatal("expected error")
			}
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("got %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestArchiveAction_WritesFinalOnlyOnSuccess(t *testing.T) {
	if _, err := exec.LookPath("tar"); err != nil {
		t.Skip("tar not available")
	}

	ctx := logging.NewWriterContext(t.Output())
	inDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(inDir, "note.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}

	outDir := t.TempDir()
	outPath := filepath.Join(outDir, "backup.tar")
	action := backup.ArchiveAction{
		InputDir: inDir,
		Output:   outPath,
		Format:   "tar",
	}
	if err := action.Run(ctx, nil); err != nil {
		t.Fatalf("archive: %v", err)
	}

	st, err := os.Stat(outPath)
	if err != nil {
		t.Fatalf("final archive missing: %v", err)
	}
	if st.Size() == 0 {
		t.Fatal("final archive is empty")
	}
	assertNoArchiveTemps(t, outDir)
}

func TestArchiveAction_FailureKeepsExistingOutput(t *testing.T) {
	if _, err := exec.LookPath("tar"); err != nil {
		t.Skip("tar not available")
	}

	ctx := logging.NewWriterContext(t.Output())
	outDir := t.TempDir()
	outPath := filepath.Join(outDir, "backup.tar")
	sentinel := []byte("previous-good-archive")
	if err := os.WriteFile(outPath, sentinel, 0o644); err != nil {
		t.Fatal(err)
	}

	action := backup.ArchiveAction{
		InputDir: filepath.Join(outDir, "does-not-exist"),
		Output:   outPath,
		Format:   "tar",
	}
	if err := action.Run(ctx, nil); err == nil {
		t.Fatal("expected archive of missing input to fail")
	}

	got, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("existing output should remain: %v", err)
	}
	if string(got) != string(sentinel) {
		t.Fatalf("existing output changed: got %q want %q", got, sentinel)
	}
	assertNoArchiveTemps(t, outDir)
}

func TestRsyncAction_RunValidation(t *testing.T) {
	t.Parallel()

	ctx := logging.NewWriterContext(t.Output())
	err := backup.RsyncAction{}.Run(ctx, nil)
	if !errors.Is(err, backup.ErrRsyncNeedsSrcAndDst) {
		t.Fatalf("got %v, want %v", err, backup.ErrRsyncNeedsSrcAndDst)
	}
}

func assertNoArchiveTemps(t *testing.T, dir string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if strings.Contains(e.Name(), ".tmp-") {
			t.Fatalf("leftover archive temp: %s", e.Name())
		}
	}
}
