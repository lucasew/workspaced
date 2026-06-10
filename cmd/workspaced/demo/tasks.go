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
			Long: `Schedules some work on a SubGroup. The actual rendering is performed by
the renderer started in the root command's PersistentPreRunE.

You will see "plain" output when the root renderer chose plain mode
(i.e. when stdout/stderr is not a tty, when TERM=dumb, CI=1, or NO_COLOR
is set). This is the same renderer (output.Plain) that the root selects
via output.Auto.`,
			RunE: func(cmd *cobra.Command, args []string) error {
				g := taskgroup.MustFromContext(cmd.Context())
				logger := logging.GetLogger(cmd.Context())
				logger.Info("Group obtained with MustFromContext (enforced rule).")
				logger.Info("Work runs under the root task group + its renderer.")
				logger.Info("Pipe the command or set TERM=dumb/CI=1 to force plain mode.")

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

				if err := g.Wait(); err != nil {
					return err
				}
				logger.Info("\n(tasks finished; root renderer (plain or interactive) drove the output above)")
				return nil
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
				return nil
			},
		})
	})
}

func init() {
	Registry.Register(func(parent *cobra.Command) {
		parent.AddCommand(&cobra.Command{
			Use:   "loop",
			Short: "Demo a 5-iteration loop (sleep + log + progress) to observe log rendering",
			RunE: func(cmd *cobra.Command, args []string) error {
				g := taskgroup.MustFromContext(cmd.Context())

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

				return nil
			},
		})
	})
}
