package demo

import (
	"context"
	"time"

	"workspaced/pkg/logging"
	"workspaced/pkg/taskgroup"

	"github.com/spf13/cobra"
)

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
				// session Close in PostRun handles UI + Wait
				return nil
			},
		})
	})
}
