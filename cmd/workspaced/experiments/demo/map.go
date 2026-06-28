package demo

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	"workspaced/pkg/logging"
	"workspaced/pkg/taskgroup"

	"github.com/spf13/cobra"
)

func init() {
	Registry.Register(func(parent *cobra.Command) {
		parent.AddCommand(&cobra.Command{
			Use:   "map",
			Short: "Demonstrate taskgroup.Map (parallel map over a list using the group system)",
			Long: `Shows the Map primitive added for the taskgroup:

- g := taskgroup.MustFromContext(cmd.Context())
- items := []string{...}                 // any slice
- results, err := taskgroup.Map(ctx, pool, items, nameFn, handler)
  - pool comes before the items list
  - len(items) is used as the progress total ("progressbar hint")
  - each item gets its own task + *Status (good names, per-item Update/Progress)
  - results are returned in the original order
  - full concurrency limiting, logging, cancellation, and Snapshot apply

The outer "map" task uses the length of the list to drive an aggregate progress bar
while child map tasks run (via internal SubGroup) with their own per-item status.

This is the recommended way to turn "for each X do Y" into group-scheduled work.`,
			RunE: func(cmd *cobra.Command, args []string) error {
				g := taskgroup.MustFromContext(cmd.Context())
				logger := logging.GetLogger(cmd.Context())

				items := []string{
					"src/main.go",
					"src/config.cue",
					"src/utils/helpers.go",
					"pkg/driver/audio/driver.go",
					"pkg/taskgroup/map.go",
					"cmd/workspaced/home/apply/root.go",
					"internal/templates/base.tmpl",
					"modules/base16/module.cue",
					"assets/icons/icon.svg",
					"README.md",
				}

				logger.Info("demonstrating taskgroup.Map", "item_count", len(items))

				// We schedule a visible parent task so the demo has something on the
				// root snapshot. Inside it we use the list length as the progress hint
				// and drive an aggregate bar while the Map runs its items.
				g.Go("map", taskgroup.CPU, func(ctx context.Context, s *taskgroup.Status) error {
					logger := logging.GetLogger(ctx)
					s.Update(fmt.Sprintf("preparing to map over %d items", len(items)))
					s.Progress(0, int64(len(items)))

					var completed atomic.Int64

					results, err := taskgroup.Map(ctx, func(string) taskgroup.PoolKind { return taskgroup.IO }, items,
						func(i int, path string) string {
							// Use the item itself for a clear, stable task name.
							// These names appear in logs and (when visible) in the UI.
							return "map:" + path
						},
						func(ctx context.Context, st *taskgroup.Status, path string) (string, error) {
							logger := logging.GetLogger(ctx)

							st.Update("starting " + path)
							st.Progress(0, 1)

							// Simulate varying work per item. Real handlers would do
							// real IO/CPU work here and report finer-grained progress.
							work := 60*time.Millisecond + time.Duration(len(path)%4)*35*time.Millisecond
							time.Sleep(work)

							logger.Info("processed item", "path", path, "result", "ok")

							// Advance the outer aggregate progress from inside the
							// map item handler. This uses len(items) as the total.
							cur := completed.Add(1)
							s.Progress(cur, int64(len(items)))
							s.Update(fmt.Sprintf("map progress: %d/%d", cur, len(items)))

							st.Progress(1, 1)
							st.Update("done " + path)

							return "processed:" + path, nil
						})

					if err != nil {
						return err
					}

					// Explicitly set 100% on the outer task's status *after* Map returns.
					// This guarantees that while the "map" task is still Running,
					// a subsequent snapshot will see Current == Total, so the
					// bubbletea bar (via ViewAs) renders at full 100% before
					// the task completes and is removed from the model.
					total := int64(len(items))
					s.Progress(total, total)
					s.Update(fmt.Sprintf("map complete — collected %d results in order", len(results)))
					logger.Info("map finished", "count", len(results), "first", results[0], "last", results[len(results)-1])
					return nil
				})

				// Opt into the bubbletea renderer (same rules as all other demos).
				return g.RunBubbleTea()
			},
		})
	})
}
