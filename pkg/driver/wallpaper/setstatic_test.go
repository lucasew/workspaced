package wallpaper

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lucasew/workspaced/pkg/api"
	_ "github.com/lucasew/workspaced/pkg/driver/exec/native"
	"github.com/lucasew/workspaced/pkg/logging"
)

func TestSetStaticViaSystemdUnit(t *testing.T) {
	ctx := logging.NewWriterContext(t.Output())
	dir := t.TempDir()
	argsPath := filepath.Join(dir, "args")
	writeExec(t, filepath.Join(dir, "systemd-run"), "#!/bin/sh\nprintf '%s\\n' \"$@\" > \""+argsPath+"\"\n")
	writeExec(t, filepath.Join(dir, "wpbin"), "#!/bin/sh\nexit 0\n")
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	if err := SetStaticViaSystemdUnit(ctx, "wpbin", "--bg-fill", "/tmp/wall.png"); err != nil {
		t.Fatalf("SetStaticViaSystemdUnit: %v", err)
	}
	got, err := os.ReadFile(argsPath)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(got)), "\n")
	wantPrefix := []string{"--user", "-u", "wallpaper-change", "--collect"}
	if len(lines) != 7 {
		t.Fatalf("args=%q want 7 lines", got)
	}
	for i, w := range wantPrefix {
		if lines[i] != w {
			t.Fatalf("args[%d]=%q want %q", i, lines[i], w)
		}
	}
	if filepath.Base(lines[4]) != "wpbin" {
		t.Fatalf("bin=%q want wpbin", lines[4])
	}
	if strings.Join(lines[5:], " ") != "--bg-fill /tmp/wall.png" {
		t.Fatalf("tail=%q want --bg-fill /tmp/wall.png", strings.Join(lines[5:], " "))
	}
}

func TestSetStaticViaSystemdUnitMissingBin(t *testing.T) {
	ctx := logging.NewWriterContext(t.Output())
	t.Setenv("PATH", t.TempDir())
	err := SetStaticViaSystemdUnit(ctx, "wpbin-missing")
	if err == nil {
		t.Fatal("expected missing binary")
	}
	if !errors.Is(err, api.ErrBinaryNotFound) {
		t.Fatalf("err=%v want ErrBinaryNotFound", err)
	}
}

func TestSetStaticViaSystemdUnitFailed(t *testing.T) {
	ctx := logging.NewWriterContext(t.Output())
	dir := t.TempDir()
	writeExec(t, filepath.Join(dir, "systemd-run"), "#!/bin/sh\nexit 1\n")
	writeExec(t, filepath.Join(dir, "wpbin"), "#!/bin/sh\nexit 0\n")
	t.Setenv("PATH", dir)
	err := SetStaticViaSystemdUnit(ctx, "wpbin", "-i", "/tmp/wall.png")
	if err == nil {
		t.Fatal("expected systemd-run failure")
	}
	var ee *exec.ExitError
	if !errors.As(err, &ee) {
		t.Fatalf("err=%v want ExitError", err)
	}
}

func writeExec(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
}
