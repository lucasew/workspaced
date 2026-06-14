package gokrazy

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"time"

	"workspaced/pkg/driver"
	rsyncdriver "workspaced/pkg/driver/rsync"
	"workspaced/pkg/logging"
	"workspaced/pkg/taskgroup"

	gokrsync "github.com/gokrazy/rsync/rsynccmd"
)

func init() {
	driver.Register[rsyncdriver.Driver](&Factory{})
}

type Factory struct{}

func (p *Factory) ID() string   { return "rsync_gokrazy" }
func (p *Factory) Name() string { return "gokrazy/rsync (pure Go)" }

func (p *Factory) CheckCompatibility(ctx context.Context) error {
	// Pure Go implementation, always available.
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

	// Capture combined output via a pipe so the scanner can drive taskgroup Status
	// (and forward lines to extraOut for notification consumers).
	pr, pw := io.Pipe()
	cmd.Stdout = pw
	cmd.Stderr = pw

	done := make(chan error, 1)
	go func() {
		_, err := cmd.Run(ctx)
		pw.Close()
		done <- err
	}()

	scanner := bufio.NewScanner(pr)
	lastUpdate := time.Now()

	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			if st != nil {
				st.Update(trimmed)
				if cur, tot, ok := parseRsyncProgress(trimmed); ok {
					st.Progress(cur, tot)
				}
			}
			if extraOut != nil {
				_, _ = io.WriteString(extraOut, line+"\n")
			}
			logger.Debug("rsync", "line", trimmed)
		}
		if time.Since(lastUpdate) > time.Second {
			lastUpdate = time.Now()
		}
	}

	// Wait for the rsynccmd.Run to finish.
	runErr := <-done
	if scanErr := scanner.Err(); scanErr != nil {
		return scanErr
	}
	return runErr
}

func parseRsyncProgress(line string) (current, total int64, ok bool) {
	if !strings.Contains(line, "%") {
		return 0, 0, false
	}
	fields := strings.Fields(line)
	for _, f := range fields {
		if strings.HasSuffix(f, "%") {
			pctStr := strings.TrimSuffix(f, "%")
			var pct int
			if _, err := fmt.Sscanf(pctStr, "%d", &pct); err == nil && pct >= 0 && pct <= 100 {
				return int64(pct), 100, true
			}
		}
	}
	return 0, 0, false
}
