package demo

import (
	"context"
	"fmt"
	"time"

	"workspaced/pkg/cmdregistry"
	"workspaced/pkg/taskgroup"

	"github.com/spf13/cobra"
)

var Registry cmdregistry.CommandRegistry

func GetCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "demo",
		Short: "Showcase the output rendering and task system",
		Long: `The demo command exercises the taskgroup execution engine and the
interactive/plain output renderers that were recently added.

Run subcommands to see different aspects:
  workspaced demo          - default: runs the tasks showcase (same as "tasks")
  workspaced demo tasks    - runs tasks using the ambient root task group (full integration)
  workspaced demo plain    - forces a plain renderer against a local group
  workspaced demo nested   - demonstrates SubGroup nesting under a parent task`,
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
	ctx := cmd.Context()
	g := taskgroup.FromContext(ctx)
	if g == nil {
		cmd.PrintErrln("warning: no taskgroup in context, creating a local one (UI may not be attached)")
		var localCtx context.Context
		g, localCtx = taskgroup.New(ctx, taskgroup.DefaultLimits())
		_ = localCtx
	}

	cmd.Println("=== workspaced demo: tasks ===")
	cmd.Println("Scheduling work on the task group. Watch the progress UI on stderr.")
	cmd.Println("Tasks use IO / CPU / Internet pools, have dependencies, emit logs, and report progress.")
	cmd.Println()

	// Internet task with determinate progress + logs.
	g.Go("download", taskgroup.Internet, func(ctx context.Context, s *taskgroup.Status) error {
		s.Update("resolving")
		time.Sleep(120 * time.Millisecond)
		s.Log("GET https://cdn.example.com/bundle.tar.gz")
		s.Progress(0, 100)
		for i := 10; i <= 100; i += 10 {
			s.Progress(int64(i), 100)
			s.Update(fmt.Sprintf("receiving %d%%", i))
			time.Sleep(70 * time.Millisecond)
			if i == 50 {
				s.Log("50% received, checking partial checksum")
			}
		}
		s.Log("download complete, sha256 verified")
		return nil
	})

	// CPU-bound work that depends on the download.
	g.Go("build", taskgroup.CPU, func(ctx context.Context, s *taskgroup.Status) error {
		s.Update("preparing sources")
		time.Sleep(80 * time.Millisecond)
		for step := 1; step <= 4; step++ {
			s.Log(fmt.Sprintf("gcc -c src/part%d.c -O2", step))
			s.Update(fmt.Sprintf("compiling part %d/4", step))
			time.Sleep(140 * time.Millisecond)
		}
		s.Update("linking")
		time.Sleep(160 * time.Millisecond)
		s.Log("build finished: ./bin/app")
		return nil
	}, "download")

	// Another CPU task in parallel with build (after download).
	g.Go("check", taskgroup.CPU, func(ctx context.Context, s *taskgroup.Status) error {
		s.Update("running static analysis")
		time.Sleep(90 * time.Millisecond)
		s.Log("golangci-lint: 0 issues")
		s.Log("govulncheck: clean")
		time.Sleep(220 * time.Millisecond)
		return nil
	}, "download")

	// IO task that depends on build.
	g.Go("install", taskgroup.IO, func(ctx context.Context, s *taskgroup.Status) error {
		s.Update("installing to $HOME/.local/bin")
		time.Sleep(60 * time.Millisecond)
		s.Log("cp ./bin/app ~/.local/bin/app")
		s.Progress(0, 1)
		time.Sleep(180 * time.Millisecond)
		s.Progress(1, 1)
		s.Log("binary installed")
		return nil
	}, "build")

	// Indeterminate task (no Total) running in parallel.
	g.Go("lint", taskgroup.CPU, func(ctx context.Context, s *taskgroup.Status) error {
		s.Update("linting workspace")
		for i := 0; i < 3; i++ {
			time.Sleep(160 * time.Millisecond)
			s.Log(fmt.Sprintf("checked package %d", i+1))
		}
		s.Update("formatting check")
		time.Sleep(120 * time.Millisecond)
		return nil
	})

	// A task that fails so the error UI is visible.
	g.Go("publish", taskgroup.Internet, func(ctx context.Context, s *taskgroup.Status) error {
		s.Update("connecting to registry")
		time.Sleep(140 * time.Millisecond)
		s.Log("POST /artifacts")
		time.Sleep(200 * time.Millisecond)
		return fmt.Errorf("simulated 503 from registry (demo failure)")
	}, "build")

	// Give the renderer a moment to paint the first frames even if we return quickly.
	time.Sleep(50 * time.Millisecond)
	return nil
}
