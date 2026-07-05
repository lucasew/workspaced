package shell_test

import (
	"path/filepath"
	"testing"

	"workspaced/pkg/driver"
	_ "workspaced/pkg/driver/exec/native"
	"workspaced/pkg/driver/shell"
	"workspaced/pkg/logging"
)

func TestWhichDriverExposesPathOnly(t *testing.T) {
	shell.RegisterWhich("shell_test_which", "Test sh", "sh")

	ctx := logging.NewWriterContext(t.Output())
	// Force the test registration regardless of weights.
	t.Setenv("WORKSPACED_FORCE_SHELL_DRIVER", "shell_test_which")

	d, err := shell.Get(ctx)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	path, err := d.Path(ctx)
	if err != nil {
		t.Fatalf("Path: %v", err)
	}
	if filepath.Base(path) != "sh" {
		t.Fatalf("Path base = %q, want sh", filepath.Base(path))
	}

	if _, ok := d.(driver.DriverFactory[shell.Driver]); ok {
		t.Fatal("shell.Driver unexpectedly implements DriverFactory")
	}
}
