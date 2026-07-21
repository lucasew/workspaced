package shim_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	_ "github.com/lucasew/workspaced/pkg/driver/prelude"
	"github.com/lucasew/workspaced/pkg/driver/shim"
	"github.com/lucasew/workspaced/pkg/logging"
)

func TestGenerateInLocalBin(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("TERMUX_VERSION", "")
	t.Setenv("TERMUX_APP_PACKAGE", "")
	t.Setenv("WORKSPACED_IN_PROOT", "")
	t.Setenv("PREFIX", "")
	ctx := logging.NewWriterContext(t.Output())
	target := filepath.Join(home, "opt", "workspaced")

	shimPath, err := shim.GenerateInLocalBin(ctx, "workspaced", []string{target})
	if err != nil {
		t.Fatalf("GenerateInLocalBin: %v", err)
	}

	wantPath := filepath.Join(home, ".local", "bin", "workspaced")
	if shimPath != wantPath {
		t.Fatalf("shim path: got %q want %q", shimPath, wantPath)
	}

	info, err := os.Stat(shimPath)
	if err != nil {
		t.Fatalf("stat shim: %v", err)
	}
	if info.Mode()&0o111 == 0 {
		t.Fatalf("shim not executable: %o", info.Mode())
	}

	content, err := os.ReadFile(shimPath)
	if err != nil {
		t.Fatalf("read shim: %v", err)
	}
	if !strings.HasPrefix(string(content), "#!") {
		t.Fatalf("missing shebang: %q", content)
	}
	if !strings.Contains(string(content), target) {
		t.Fatalf("shim missing target %q:\n%s", target, content)
	}
}

func TestGenerateInLocalBinValidation(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	ctx := logging.NewWriterContext(t.Output())

	if _, err := shim.GenerateInLocalBin(ctx, "", []string{"/bin/true"}); !errors.Is(err, shim.ErrEmptyName) {
		t.Fatalf("empty name: got %v want %v", err, shim.ErrEmptyName)
	}
	if _, err := shim.GenerateInLocalBin(ctx, "workspaced", nil); !errors.Is(err, shim.ErrEmptyCommand) {
		t.Fatalf("empty command: got %v want %v", err, shim.ErrEmptyCommand)
	}
	if _, err := shim.GenerateInLocalBin(ctx, "../workspaced", []string{"/bin/true"}); err == nil {
		t.Fatal("expected error for non-base shim name")
	}
}

func TestLocalBinDir(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	// Ensure Termux rewrite does not apply in unit tests.
	t.Setenv("TERMUX_VERSION", "")
	t.Setenv("TERMUX_APP_PACKAGE", "")
	t.Setenv("WORKSPACED_IN_PROOT", "")
	t.Setenv("PREFIX", "")

	got, err := shim.LocalBinDir()
	if err != nil {
		t.Fatalf("LocalBinDir: %v", err)
	}
	want := filepath.Join(home, ".local", "bin")
	if got != want {
		t.Fatalf("LocalBinDir: got %q want %q", got, want)
	}
}

// HOME under /home/<user> must not be re-prefixed when writing shims (the old
// /home/ string rewrite turned /home/user/... into /home/user/user/...).
func TestGenerateInLocalBinKeepsNormalHomePaths(t *testing.T) {
	// Use a path shaped like a real Linux home.
	root := t.TempDir()
	home := filepath.Join(root, "home", "user")
	if err := os.MkdirAll(home, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
	t.Setenv("TERMUX_VERSION", "")
	t.Setenv("TERMUX_APP_PACKAGE", "")
	t.Setenv("WORKSPACED_IN_PROOT", "")
	t.Setenv("PREFIX", "")

	target := filepath.Join(home, ".local", "share", "workspaced", "bin", "workspaced")
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatal(err)
	}
	ctx := logging.NewWriterContext(t.Output())
	shimPath, err := shim.GenerateInLocalBin(ctx, "workspaced", []string{target})
	if err != nil {
		t.Fatalf("GenerateInLocalBin: %v", err)
	}
	content, err := os.ReadFile(shimPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), target) {
		t.Fatalf("shim missing exact target %q:\n%s", target, content)
	}
	doubled := filepath.Join(home, "user")
	if strings.Contains(string(content), doubled) {
		t.Fatalf("shim re-prefixed home (doubled user path):\n%s", content)
	}
}
