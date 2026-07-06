package exec

import (
	"context"
	"fmt"
	"os"
	"os/exec"

	"workspaced/pkg/driver"
	"workspaced/pkg/executil"
	"workspaced/pkg/logging"
)

// Driver provides platform-specific command execution.
type Driver interface {
	// Run creates an exec.Cmd configured for the platform.
	Run(ctx context.Context, name string, args ...string) *exec.Cmd

	// Which locates a command in PATH and returns its full path.
	Which(ctx context.Context, name string) (string, error)
}

// IsBinaryAvailable checks if a command exists in PATH using the selected driver.
func IsBinaryAvailable(ctx context.Context, name string) bool {
	d, err := driver.Get[Driver](ctx)
	if err != nil {
		return false
	}
	_, err = d.Which(ctx, name)
	return err == nil
}

// Run creates an exec.Cmd using the selected driver.
// Stderr defaults to the process os.Stderr (including session UI redirects).
// Context writers from executil override stdout/stderr when set.
func Run(ctx context.Context, name string, args ...string) (*exec.Cmd, error) {
	logger := logging.GetLogger(ctx)
	logger.Debug("running command", "name", name, "args", args)
	d, err := driver.Get[Driver](ctx)
	if err != nil {
		return nil, err
	}
	cmd := d.Run(ctx, name, args...)
	attachDefaultWriters(ctx, cmd)
	return cmd, nil
}

// Which locates a command in PATH using the selected driver.
func Which(ctx context.Context, name string) (string, error) {
	d, err := driver.Get[Driver](ctx)
	if err != nil {
		return "", err
	}
	return d.Which(ctx, name)
}

// MustRun creates and returns an exec.Cmd using the selected driver.
// Panics if the driver cannot be loaded (should only happen during initialization).
// Use this for compatibility with code that expects *exec.Cmd directly.
func MustRun(ctx context.Context, name string, args ...string) *exec.Cmd {
	cmd, err := Run(ctx, name, args...)
	if err != nil {
		// Fallback to direct exec if driver fails
		cmd = exec.CommandContext(ctx, name, args...)
		attachDefaultWriters(ctx, cmd)
		return cmd
	}
	return cmd
}

// attachDefaultWriters sets stderr to the process os.Stderr so Output() does
// not swallow diagnostics, then applies executil context overrides when set.
// Stdout is left unset unless the context carries one, so callers can still
// use Cmd.Output() or assign a capture buffer.
func attachDefaultWriters(ctx context.Context, cmd *exec.Cmd) {
	cmd.Stderr = os.Stderr
	if stdout := executil.Stdout(ctx); stdout != nil {
		cmd.Stdout = stdout
	}
	if stderr := executil.Stderr(ctx); stderr != nil {
		cmd.Stderr = stderr
	}
}

// RequireBinary returns driver.ErrIncompatible when name is missing from PATH.
func RequireBinary(ctx context.Context, name string) error {
	if IsBinaryAvailable(ctx, name) {
		return nil
	}
	return fmt.Errorf("%w: %s not found", driver.ErrIncompatible, name)
}

// RequireBinaries returns the first RequireBinary error for names.
func RequireBinaries(ctx context.Context, names ...string) error {
	for _, name := range names {
		if err := RequireBinary(ctx, name); err != nil {
			return err
		}
	}
	return nil
}
