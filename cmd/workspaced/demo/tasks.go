package demo

import (
	"context"
	"fmt"
	"time"

	"workspaced/pkg/logging"
	"workspaced/pkg/taskgroup"

	"github.com/spf13/cobra"
)

func init() {
	Registry.Register(func(parent *cobra.Command) {
		parent.AddCommand(&cobra.Command{
			Use:   "tasks",
			Short: "Run a set of tasks that demonstrate progress bars, logs, pools and dependencies",
			RunE: func(cmd *cobra.Command, args []string) error {
				return runTasksDemo(cmd)
			},
		})
	})
}

func init() {
	Registry.Register(func(parent *cobra.Command) {
		parent.AddCommand(&cobra.Command{
			Use:   "plain",
			Short: "Run tasks under the root group; observe plain-style rendering behavior",
			Long: `Schedules some work on the group from context but deliberately does NOT
call g.RunBubbleTea().

This produces plain structured slog output for the logs emitted from inside
the tasks (via logging.GetLogger(ctx).Info etc). To force this plain path
on a tty you can set TERM=dumb (or CI=1 or NO_COLOR).

This demonstrates the default (no TUI) behavior that all non-demo commands
use: they schedule work via the primitives and never start bubbletea.`,
			RunE: func(cmd *cobra.Command, args []string) error {
				g := taskgroup.MustFromContext(cmd.Context())
				logger := logging.GetLogger(cmd.Context())
				logger.Info("Group obtained with MustFromContext (enforced rule).")
				logger.Info("This demo does not call RunBubbleTea, so you get plain slog output.")
				logger.Info("Pipe the command or set TERM=dumb/CI=1/NO_COLOR to observe plain behavior on tty.")

				g.Go("fetch", taskgroup.Internet, func(ctx context.Context, s *taskgroup.Status) error {
					logger := logging.GetLogger(ctx)
					s.Update("contacting API")
					time.Sleep(80 * time.Millisecond)
					logger.Info("http response", "status", "200 OK")
					s.Progress(0, 4)
					for i := 1; i <= 4; i++ {
						s.Progress(int64(i), 4)
						s.Update(fmt.Sprintf("page %d", i))
						time.Sleep(90 * time.Millisecond)
					}
					return nil
				})

				g.Go("process", taskgroup.CPU, func(ctx context.Context, s *taskgroup.Status) error {
					logger := logging.GetLogger(ctx)
					s.Update("crunching numbers")
					for i := range 3 {
						logger.Info("batch processed", "num", i)
						time.Sleep(110 * time.Millisecond)
					}
					return nil
				}, "fetch")

				g.Go("write", taskgroup.IO, func(ctx context.Context, s *taskgroup.Status) error {
					logger := logging.GetLogger(ctx)
					s.Update("writing artifacts")
					time.Sleep(150 * time.Millisecond)
					logger.Info("fsync complete")
					return nil
				}, "process")

				return g.RunBubbleTea()
			},
		})
	})
}

func init() {
	Registry.Register(func(parent *cobra.Command) {
		parent.AddCommand(&cobra.Command{
			Use:   "nested",
			Short: "Demonstrate a parent task owning a SubGroup with child tasks",
			RunE: func(cmd *cobra.Command, args []string) error {
				g := taskgroup.MustFromContext(cmd.Context())
				logger := logging.GetLogger(cmd.Context())

				logger.Info("We get the group with MustFromContext, then create a SubGroup from it.")
				logger.Info("Child tasks share pools with the parent but live in a separate snapshot.")

				g.Go("bundle", taskgroup.CPU, func(ctx context.Context, s *taskgroup.Status) error {
					s.Update("starting bundle phase")
					time.Sleep(60 * time.Millisecond)

					child, childCtx := g.SubGroup(ctx)
					_ = childCtx
					child.Go("bundle:icons", taskgroup.CPU, func(ctx context.Context, s *taskgroup.Status) error {
						logger := logging.GetLogger(ctx)
						s.Update("generating icons")
						for i := 0; i < 3; i++ {
							logger.Info("icon", "num", i)
							time.Sleep(90 * time.Millisecond)
						}
						return nil
					})
					child.Go("bundle:manifest", taskgroup.IO, func(ctx context.Context, s *taskgroup.Status) error {
						logger := logging.GetLogger(ctx)
						s.Update("writing manifest.json")
						time.Sleep(130 * time.Millisecond)
						logger.Info("manifest written")
						return nil
					}, "bundle:icons")

					s.Update("waiting on subgroup children (not directly visible on root snapshot)")
					if err := child.Wait(); err != nil {
						return err
					}
					s.Update("bundle complete (children ran in SubGroup)")
					logger := logging.GetLogger(ctx)
					logger.Info("subgroup done")
					return nil
				})

				time.Sleep(30 * time.Millisecond)

				// Showcase: opt into bubbletea renderer via the group method.
				// Child subgroup bars are not on the parent snapshot (by design),
				// but logs still flow and the parent "bundle" task will show a bar.
				_ = g.RunBubbleTea()
				return nil
			},
		})
	})
}

func init() {
	Registry.Register(func(parent *cobra.Command) {
		parent.AddCommand(&cobra.Command{
			Use:   "loop",
			Short: "Demo a 5-iteration loop (like the other demos: schedule on group from context and return)",
			Long: `Uses exactly the same primitives as "tasks", "plain", and "nested":
- g := taskgroup.MustFromContext(cmd.Context())
- g.Go("loop-demo", ..., func(ctx, s) { logger := ...; logger.Info(...); s.Update(...); s.Progress(...) })
- g.RunBubbleTea()  (opt-in; the group method starts the bubbletea UI for this demo
  only, and is a no-op on TERM=dumb / non-tty. Other commands never call it.)`,
			RunE: func(cmd *cobra.Command, args []string) error {
				g := taskgroup.MustFromContext(cmd.Context())

				// Schedule using the task primitive (same as all other demos).
				g.Go("loop-demo", taskgroup.CPU, func(ctx context.Context, s *taskgroup.Status) error {
					logger := logging.GetLogger(ctx)
					for i := 1; i <= 5; i++ {
						time.Sleep(1 * time.Second)
						logger.Info("log line from loop", "iteration", i)
						s.Update(fmt.Sprintf("step %d/5", i))
						s.Progress(int64(i), 5)
					}
					return nil
				})

				// Kick in bubbletea (group method). Ignored automatically on dumb term.
				// This is what makes the demo show live bars + logs scrolling above them.
				return g.RunBubbleTea()
			},
		})
	})
}
