package demo

import (
	"context"
	"time"

	"github.com/lucasew/workspaced/pkg/logging"
	"github.com/lucasew/workspaced/pkg/taskgroup"

	"github.com/spf13/cobra"
)

func init() {
	Registry.Register(func(parent *cobra.Command) {
		parent.AddCommand(&cobra.Command{
			Use:   "nested",
			Short: "Demonstrate Isolate as an error-boundary SubGroup with child tasks",
			Long: `Isolate runs work on a child Group that shares pools with the parent but
does not cancel parent siblings on failure and does not add its own progress bar.

Schedule named tasks inside the isolated ctx (or use GoIsolated / Map). The TUI
sees child tasks via snapshotRecursive. Prefer Map when you need aggregate progress.`,
			RunE: func(cmd *cobra.Command, args []string) error {
				g := taskgroup.MustFromContext(cmd.Context())
				logger := logging.GetLogger(cmd.Context())
				logger.Info("scheduling bundle with Isolate children")

				g.Go("bundle", taskgroup.Control, func(ctx context.Context, s *taskgroup.Status) error {
					s.Update("starting bundle phase")
					time.Sleep(60 * time.Millisecond)
					err := taskgroup.Isolate(ctx, func(ctx context.Context) error {
						child := taskgroup.MustFromContext(ctx)
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
						return child.Wait()
					})
					if err != nil {
						return err
					}
					s.Update("bundle complete")
					logging.GetLogger(ctx).Info("isolate subgroup done")
					return nil
				})
				return nil
			},
		})
	})
}
