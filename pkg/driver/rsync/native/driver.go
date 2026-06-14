package native

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"time"

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
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	cmd.Stderr = cmd.Stdout

	if err := cmd.Start(); err != nil {
		return err
	}

	scanner := bufio.NewScanner(stdout)
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

	waitErr := cmd.Wait()
	if scanErr := scanner.Err(); scanErr != nil {
		return scanErr
	}
	return waitErr
}

func parseRsyncProgress(line string) (current, total int64, ok bool) {
	// Look for percent in typical rsync -P / --info=progress output.
	// Examples: "1,234,567  42%  123.45kB/s    0:00:12"
	// We map % to a 0-100 progress bar (good enough for the TUI; absolute file size
	// totals require more parsing of rsync headers).
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
