package gokrazy

import (
	"context"
	"io"
	"log/slog"
	"os"
	"strings"

	"github.com/lucasew/workspaced/pkg/driver"
	rsyncdriver "github.com/lucasew/workspaced/pkg/driver/rsync"
	"github.com/lucasew/workspaced/pkg/logging"
	"github.com/lucasew/workspaced/pkg/taskgroup"

	gokrsync "github.com/gokrazy/rsync/rsynccmd"
)

func init() {
	driver.Register[rsyncdriver.Driver](&Factory{})
}

type Factory struct{}

func (f *Factory) ID() string   { return "rsync_gokrazy" }
func (f *Factory) Name() string { return "gokrazy/rsync (pure Go)" }

func (f *Factory) CheckCompatibility(ctx context.Context) error {
	// Pure Go implementation, always available.
	return nil
}

func (f *Factory) New(ctx context.Context) (rsyncdriver.Driver, error) {
	return &Driver{}, nil
}

type Driver struct{}

func (d *Driver) Sync(ctx context.Context, src, dst string, opts rsyncdriver.Options) error {
	if strings.TrimSpace(src) == "" || strings.TrimSpace(dst) == "" {
		return rsyncdriver.ErrNeedsSrcAndDst
	}
	logger := logging.GetLogger(ctx)

	// Build args for the gokrazy reimplementation.
	//
	// gokrazy/rsync uses a custom limited popt-style parser (internal/rsyncopts).
	// Only options registered in gokrazyTable() are recognized at runtime.
	// Several things (including -P and --partial) are commented out as
	// "not yet implemented".
	//
	// Safe currently-active flags we can rely on:
	//   -a, -v, --progress (long), --exclude, --no-perms
	//
	// We avoid -P and --partial here. Progress/partial transfer behavior
	// is best-effort from the library itself + our own taskgroup status.
	extraArgs := make([]string, 0, len(opts.Excludes))
	for _, x := range opts.Excludes {
		extraArgs = append(extraArgs, "--exclude="+x)
	}
	if opts.SkipPermissions {
		extraArgs = append(extraArgs, "--no-perms")
	}
	args := append(extraArgs, "-av", "--progress", src, dst)

	perform := func(ctx context.Context, st *taskgroup.Status, extraOut io.Writer) error {
		return d.runRsyncCmd(ctx, args, st, extraOut, logger)
	}

	return rsyncdriver.RunWithTaskGroup(ctx, src, dst, opts, perform)
}

func (d *Driver) runRsyncCmd(ctx context.Context, args []string, st *taskgroup.Status, extraOut io.Writer, logger *slog.Logger) error {
	// rsynccmd gives us a drop-in replacement for spawning rsync.
	cmd := gokrsync.Command("rsync", args...)

	// Use the real process stdout/stderr directly (standard terminal behavior).
	// No pipes, no line scanning, no progress extraction inside the driver.
	// rsync's own output (including --progress chatter, file lists, errors, etc.)
	// will appear on the caller's terminal exactly as a normal rsync invocation
	// would. This is the requested "no fancy stuff" behavior.
	// Both streams to stderr (user request: no fancy capture, rsync should
	// behave like a plain command but with all its chatter on stderr).
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr

	_, err := cmd.Run(ctx)
	return err
}
