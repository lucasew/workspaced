package native

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"

	"github.com/lucasew/workspaced/pkg/driver"
	execdriver "github.com/lucasew/workspaced/pkg/driver/exec"
	rsyncdriver "github.com/lucasew/workspaced/pkg/driver/rsync"
	"github.com/lucasew/workspaced/pkg/logging"
	"github.com/lucasew/workspaced/pkg/taskgroup"
)

// ErrBinaryNotAvailable is returned when execRsync runs without an rsync binary on PATH.
var ErrBinaryNotAvailable = errors.New("rsync binary not available")

func init() {
	driver.Register[rsyncdriver.Driver](&Factory{})
}

type Factory struct{}

func (f *Factory) ID() string   { return "rsync_native" }
func (f *Factory) Name() string { return "Native rsync" }

func (f *Factory) CheckCompatibility(ctx context.Context) error {
	return execdriver.RequireBinary(ctx, "rsync")
}

func (f *Factory) New(ctx context.Context) (rsyncdriver.Driver, error) {
	return &Driver{}, nil
}

type Driver struct{}

func (d *Driver) Sync(ctx context.Context, src, dst string, opts rsyncdriver.Options) error {
	if err := rsyncdriver.ValidatePaths(src, dst); err != nil {
		return err
	}
	logger := logging.GetLogger(ctx)

	// Build args in Sync so the perform closure (passed to RunWithTaskGroup) can capture everything it needs.
	args := rsyncdriver.BuildCLIArgs(opts, []string{"-avP"}, src, dst)

	perform := func(ctx context.Context, st *taskgroup.Status, extraOut io.Writer) error {
		return d.execRsync(ctx, args, st, extraOut, logger)
	}

	return rsyncdriver.RunWithTaskGroup(ctx, src, dst, opts, perform)
}

func (d *Driver) execRsync(ctx context.Context, args []string, st *taskgroup.Status, extraOut io.Writer, logger *slog.Logger) error {
	if !execdriver.IsBinaryAvailable(ctx, "rsync") {
		return ErrBinaryNotAvailable
	}

	cmd := execdriver.MustRun(ctx, "rsync", args...)

	// Use the real process stdout/stderr directly (standard terminal behavior).
	// No pipes, no line scanning, no progress extraction inside the driver.
	// rsync's own output (including -P/--progress chatter, file lists, errors,
	// etc.) will appear on the caller's terminal exactly as a normal rsync
	// invocation would. This is the requested "no fancy stuff" behavior,
	// especially important on Termux where the taskgroup renderer + capture
	// was swallowing output.
	// Both streams to stderr (user request: no fancy capture, rsync should
	// behave like a plain command but with all its chatter on stderr).
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr

	return cmd.Run()
}
