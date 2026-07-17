package native

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"

	"workspaced/pkg/driver"
	execdriver "workspaced/pkg/driver/exec"
	rsyncdriver "workspaced/pkg/driver/rsync"
	"workspaced/pkg/logging"
	"workspaced/pkg/taskgroup"
)

func init() {
	driver.Register[rsyncdriver.Driver](&Factory{})
}

type Factory struct{}

func (p *Factory) ID() string   { return "rsync_native" }
func (p *Factory) Name() string { return "Native rsync" }

func (p *Factory) CheckCompatibility(ctx context.Context) error {
	if !execdriver.IsBinaryAvailable(ctx, "rsync") {
		return fmt.Errorf("%w: rsync", driver.ErrIncompatible)
	}
	return nil
}

func (p *Factory) New(ctx context.Context) (rsyncdriver.Driver, error) {
	return &Driver{}, nil
}

type Driver struct{}

func (d *Driver) Sync(ctx context.Context, src, dst string, opts rsyncdriver.Options) error {
	if strings.TrimSpace(src) == "" || strings.TrimSpace(dst) == "" {
		return rsyncdriver.ErrNeedsSrcAndDst
	}
	logger := logging.GetLogger(ctx)

	// Build args in Sync so the perform closure (passed to RunWithTaskGroup) can capture everything it needs.
	extraArgs := make([]string, 0, len(opts.Excludes))
	for _, x := range opts.Excludes {
		extraArgs = append(extraArgs, "--exclude="+x)
	}
	if opts.SkipPermissions {
		extraArgs = append(extraArgs, "--no-perms")
	}
	args := append(extraArgs, "-avP", src, dst)

	perform := func(ctx context.Context, st *taskgroup.Status, extraOut io.Writer) error {
		return d.execRsync(ctx, args, st, extraOut, logger)
	}

	return rsyncdriver.RunWithTaskGroup(ctx, src, dst, opts, perform)
}

func (d *Driver) execRsync(ctx context.Context, args []string, st *taskgroup.Status, extraOut io.Writer, logger *slog.Logger) error {
	if !execdriver.IsBinaryAvailable(ctx, "rsync") {
		return fmt.Errorf("rsync binary not available")
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
