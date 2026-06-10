package demo

import (
	"context"
	"fmt"
	"time"

	"workspaced/pkg/cmdregistry"
	"workspaced/pkg/logging"
	"workspaced/pkg/taskgroup"

	"github.com/spf13/cobra"
)

var Registry cmdregistry.CommandRegistry

func GetCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "demo",
		Short: "Showcase the output rendering and task system",
		Long: `The demo command exercises the taskgroup primitive (g.Go + Status + context slog)
and the opt-in bubbletea renderer (a Group method).

All demos use the exact same rules as production code:
- only root may New the group; everything else does MustFromContext
- schedule with g.Go(..., func(ctx, s){ logger:=logging.GetLogger(ctx); ... s.Update/Progress })
- bubbletea UI is opt-in via g.RunBubbleTea() (demos call it; normal cmds never do)
- RunBubbleTea is a no-op (plain Wait + normal slog) when TERM=dumb / CI / non-tty

Run subcommands to see different aspects:
  workspaced demo          - default tasks showcase (calls RunBubbleTea)
  workspaced demo tasks    - same as above
  workspaced demo plain    - schedules but does NOT call RunBubbleTea (plain transcript)
  workspaced demo nested   - SubGroup + explicit RunBubbleTea
  workspaced demo loop     - 5x sleep+log+progress; calls RunBubbleTea to show logs over moving bar`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Bare "demo" runs the main tasks showcase for convenience.
			return runTasksDemo(cmd)
		},
	}
	Registry.FillCommands(cmd)
	return cmd
}

// runTasksDemo is the body used by both bare "demo" and "demo tasks".
func runTasksDemo(cmd *cobra.Command) error {
	// All non-root code must get the group from the context provided by the
	// top-level command. MustFromContext panics if it is absent.
	g := taskgroup.MustFromContext(cmd.Context())
	logger := logging.GetLogger(cmd.Context())

	logger.Info("Scheduling work on the task group obtained via MustFromContext.")
	logger.Info("This demo calls g.RunBubbleTea() to kick in the (opt-in) bubbletea UI.")
	logger.Info("Tasks use IO / CPU / Internet pools, have dependencies, emit logs, and report progress.")

	// Internet task with determinate progress + logs.
	g.Go("download", taskgroup.Internet, func(ctx context.Context, s *taskgroup.Status) error {
		logger := logging.GetLogger(ctx)
		logger.Info("starting download")

		s.Update("resolving")
		time.Sleep(120 * time.Millisecond)
		logger.Info("GET", "url", "https://cdn.example.com/bundle.tar.gz")
		s.Progress(0, 100)
		for i := 10; i <= 100; i += 10 {
			s.Progress(int64(i), 100)
			s.Update(fmt.Sprintf("receiving %d%%", i))
			time.Sleep(70 * time.Millisecond)
			if i == 50 {
				logger.Info("50% received, checking partial checksum")
			}
		}
		logger.Info("download complete", "sha256", "verified")
		return nil
	})

	// CPU-bound work that depends on the download.
	g.Go("build", taskgroup.CPU, func(ctx context.Context, s *taskgroup.Status) error {
		logger := logging.GetLogger(ctx)
		s.Update("preparing sources")
		time.Sleep(80 * time.Millisecond)
		for step := 1; step <= 4; step++ {
			logger.Info("gcc -c", "src", fmt.Sprintf("part%d.c", step), "opt", "-O2")
			s.Update(fmt.Sprintf("compiling part %d/4", step))
			time.Sleep(140 * time.Millisecond)
		}
		s.Update("linking")
		time.Sleep(160 * time.Millisecond)
		logger.Info("build finished", "binary", "./bin/app")
		return nil
	}, "download")

	// Another CPU task in parallel with build (after download).
	g.Go("check", taskgroup.CPU, func(ctx context.Context, s *taskgroup.Status) error {
		logger := logging.GetLogger(ctx)
		s.Update("running static analysis")
		time.Sleep(90 * time.Millisecond)
		logger.Info("golangci-lint", "issues", 0)
		logger.Info("govulncheck", "status", "clean")
		time.Sleep(220 * time.Millisecond)
		return nil
	}, "download")

	// IO task that depends on build.
	g.Go("install", taskgroup.IO, func(ctx context.Context, s *taskgroup.Status) error {
		logger := logging.GetLogger(ctx)
		s.Update("installing to $HOME/.local/bin")
		time.Sleep(60 * time.Millisecond)
		logger.Info("cp", "src", "./bin/app", "dst", "~/.local/bin/app")
		s.Progress(0, 1)
		time.Sleep(180 * time.Millisecond)
		s.Progress(1, 1)
		logger.Info("binary installed")
		return nil
	}, "build")

	// Indeterminate task (no Total) running in parallel.
	g.Go("lint", taskgroup.CPU, func(ctx context.Context, s *taskgroup.Status) error {
		logger := logging.GetLogger(ctx)
		s.Update("linting workspace")
		for i := 0; i < 3; i++ {
			time.Sleep(160 * time.Millisecond)
			logger.Info("checked package", "num", i+1)
		}
		s.Update("formatting check")
		time.Sleep(120 * time.Millisecond)
		return nil
	})

	// A task that fails so the error UI is visible.
	g.Go("publish", taskgroup.Internet, func(ctx context.Context, s *taskgroup.Status) error {
		logger := logging.GetLogger(ctx)
		s.Update("connecting to registry")
		time.Sleep(140 * time.Millisecond)
		logger.Info("POST", "path", "/artifacts")
		time.Sleep(200 * time.Millisecond)
		return fmt.Errorf("simulated 503 from registry (demo failure)")
	}, "build")

	// Opt-in to the bubbletea renderer (the group method). This is ignored
	// automatically if TERM=dumb / non-tty / CI. Normal commands never call
	// this, so bubbletea does not run for them.
	_ = g.RunBubbleTea()
	return nil
}
