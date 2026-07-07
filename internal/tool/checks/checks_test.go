package checks

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"workspaced/internal/modfile"
	"workspaced/internal/tool/backend"
)

func TestBinaryCandidates(t *testing.T) {
	t.Parallel()
	base := filepath.Join("tools", "pkg", "1.0.0")
	got := BinaryCandidates(base, "gh")
	want := []string{
		filepath.Join(base, "bin", "gh"),
		filepath.Join(base, "bin", "gh.exe"),
		filepath.Join(base, "bin", "gh.cmd"),
		filepath.Join(base, "bin", "gh.bat"),
		filepath.Join(base, "gh"),
		filepath.Join(base, "gh.exe"),
		filepath.Join(base, "gh.cmd"),
		filepath.Join(base, "gh.bat"),
	}
	if len(got) != len(want) {
		t.Fatalf("len = %d, want %d", len(got), len(want))
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestFileExists(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "bin", "tool")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	ctx := t.Context()
	if err := FileExists("bin/tool").Check(ctx, dir); err != nil {
		t.Fatalf("FileExists: %v", err)
	}
	if err := FileExists("bin/missing").Check(ctx, dir); err == nil {
		t.Fatal("expected error for missing file")
	}
	if err := FileExists("../outside").Check(ctx, dir); err == nil {
		t.Fatal("expected error for path escape")
	}
}

func TestExecutable(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	binDir := filepath.Join(dir, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	execPath := filepath.Join(binDir, "tool")
	plainPath := filepath.Join(binDir, "plain")
	if err := os.WriteFile(execPath, []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(plainPath, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	ctx := t.Context()
	if err := Executable("bin/tool").Check(ctx, dir); err != nil {
		t.Fatalf("Executable on 0755: %v", err)
	}
	err := Executable("bin/plain").Check(ctx, dir)
	if runtime.GOOS == "windows" {
		if err != nil {
			t.Fatalf("windows Executable on non-dir: %v", err)
		}
	} else if err == nil {
		t.Fatal("expected error for non-executable file")
	}
}

func TestBinaryComposition(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "bin", "foo")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}

	ctx := t.Context()
	if err := Binary("foo").Check(ctx, dir); err != nil {
		t.Fatalf("Binary(foo) in bin/: %v", err)
	}

	top := t.TempDir()
	topPath := filepath.Join(top, "foo")
	if err := os.WriteFile(topPath, []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := Binary("foo").Check(ctx, top); err != nil {
		t.Fatalf("Binary(foo) at top level: %v", err)
	}

	empty := t.TempDir()
	if err := Binary("foo").Check(ctx, empty); err == nil {
		t.Fatal("expected ErrBinaryNotFound")
	} else if !errors.Is(err, ErrBinaryNotFound) {
		t.Fatalf("error = %v, want ErrBinaryNotFound", err)
	}
}

type stubTool struct {
	checks []Check
}

func (stubTool) ListVersions(context.Context) ([]string, error) { return nil, nil }
func (stubTool) Install(context.Context, string, string) error  { return nil }
func (stubTool) EnrichLockfile(*modfile.RenovateDependency)     {}
func (s stubTool) InstallChecks() []Check                       { return s.checks }

func TestRunNoopWithoutChecker(t *testing.T) {
	t.Parallel()
	var tNoCheck backend.Tool = stubTool{}
	// stubTool implements InstallChecker with nil checks — treat as empty.
	if err := Run(t.Context(), t.TempDir(), stubTool{}); err != nil {
		t.Fatal(err)
	}
	type plain struct{ stubTool }
	p := plain{}
	p.checks = nil
	// plain embeds stubTool so still implements InstallChecker.
	_ = tNoCheck
}

type noChecker struct{}

func (noChecker) ListVersions(context.Context) ([]string, error) { return nil, nil }
func (noChecker) Install(context.Context, string, string) error  { return nil }
func (noChecker) EnrichLockfile(*modfile.RenovateDependency)     {}

func TestRunNoopOnNonChecker(t *testing.T) {
	t.Parallel()
	if err := Run(t.Context(), t.TempDir(), noChecker{}); err != nil {
		t.Fatal(err)
	}
}

func TestRunJoinsFailures(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	err := Run(t.Context(), dir, stubTool{checks: Checks(
		FileExists("a"),
		FileExists("b"),
	)})
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, ErrFailed) {
		t.Fatalf("error = %v, want ErrFailed", err)
	}
}

func TestRunRespectsContextCancel(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	dir := t.TempDir()
	path := filepath.Join(dir, "bin", "x")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}
	err := Run(ctx, dir, stubTool{checks: Checks(Binary("x"))})
	if err == nil {
		t.Fatal("expected context error")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
}

func TestFindBinary(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "bin", "rg")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}
	got := FindBinary(dir, "rg")
	if got != path {
		t.Fatalf("FindBinary = %q, want %q", got, path)
	}
	if FindBinary(dir, "missing") != "" {
		t.Fatal("expected empty for missing binary")
	}
}

func TestEnsureBinary(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "bin", "tool")
	installed := false
	got, err := EnsureBinary(dir, "tool", "TestTool", func() error {
		installed = true
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		return os.WriteFile(path, []byte("x"), 0o755)
	})
	if err != nil {
		t.Fatalf("EnsureBinary: %v", err)
	}
	if !installed {
		t.Fatal("expected install callback to run")
	}
	if got != path {
		t.Fatalf("EnsureBinary = %q, want %q", got, path)
	}

	_, err = EnsureBinary(t.TempDir(), "missing", "TestTool", func() error { return nil })
	if err == nil {
		t.Fatal("expected error when binary missing after install")
	}
}
