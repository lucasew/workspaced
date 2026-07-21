package env_test

import (
	"path/filepath"
	"testing"

	"github.com/lucasew/workspaced/pkg/driver/env"
)

func TestNormalizeHomeTermuxChroot(t *testing.T) {
	t.Setenv("TERMUX_VERSION", "0.118.3")
	t.Setenv("PREFIX", "/data/data/com.termux/files/usr")
	t.Setenv("HOME", "/home")

	got := env.NormalizeHome("/home")
	want := "/data/data/com.termux/files/home"
	if got != want {
		t.Fatalf("NormalizeHome(/home) = %q, want %q", got, want)
	}
}

func TestNormalizeHomeTermuxAlreadyAbsolute(t *testing.T) {
	t.Setenv("TERMUX_VERSION", "0.118.3")
	t.Setenv("PREFIX", "/data/data/com.termux/files/usr")
	realHome := "/data/data/com.termux/files/home"
	got := env.NormalizeHome(realHome)
	if got != realHome {
		t.Fatalf("NormalizeHome(real) = %q, want %q", got, realHome)
	}
}

func TestNormalizeHomeNonTermuxLeavesHome(t *testing.T) {
	t.Setenv("TERMUX_VERSION", "")
	t.Setenv("TERMUX_APP_PACKAGE", "")
	t.Setenv("WORKSPACED_IN_PROOT", "")
	t.Setenv("PREFIX", "/usr")
	got := env.NormalizeHome("/home")
	if got != "/home" {
		t.Fatalf("NormalizeHome on non-Termux = %q, want /home", got)
	}
}

func TestResolveHomeDirTermux(t *testing.T) {
	t.Setenv("TERMUX_VERSION", "0.118.3")
	t.Setenv("PREFIX", "/data/data/com.termux/files/usr")
	t.Setenv("HOME", "/home")

	got, err := env.ResolveHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join("/data/data/com.termux/files", "home")
	if got != want {
		t.Fatalf("ResolveHomeDir = %q, want %q", got, want)
	}
}

func TestIsTermuxLike(t *testing.T) {
	t.Setenv("TERMUX_VERSION", "")
	t.Setenv("TERMUX_APP_PACKAGE", "")
	t.Setenv("WORKSPACED_IN_PROOT", "")
	t.Setenv("PREFIX", "/usr")
	if env.IsTermuxLike() {
		t.Fatal("expected false without markers")
	}

	t.Setenv("PREFIX", "/data/data/com.termux/files/usr")
	if !env.IsTermuxLike() {
		t.Fatal("expected true with Termux PREFIX")
	}
}
