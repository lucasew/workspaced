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
			Use:   "map",
			Short: "Demonstrate taskgroup.Map.Run (parallel map over a list)",
			Long: `Shows the Map struct:

  results, err := taskgroup.Map[string, string]{
      Name: "demo-map",
      Items: items,
      PoolKind: taskgroup.IO,
      TaskName: nameFn,
      Fn: handler,
  }.Run(ctx)

Map.Run owns one aggregate Control bar. Do not wrap it in another Control+Progress
parent or call Status.Unit on children — that duplicates the progress hierarchy.

Children may Update messages and report multi-step Progress when useful; the
orchestrator tracks completed/total automatically.`,
			RunE: func(cmd *cobra.Command, args []string) error {
				ctx := cmd.Context()
				logger := logging.GetLogger(ctx)

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

				logger.Info("demonstrating taskgroup.Map.Run", "item_count", len(items))

				results, err := taskgroup.Map[string, string]{
					Name:     "demo-map",
					Items:    items,
					PoolKind: taskgroup.IO,
					TaskName: func(_ int, path string) string { return "item:" + path },
					Fn: func(ctx context.Context, st *taskgroup.Status, path string) (string, error) {
						logger := logging.GetLogger(ctx)
						st.Update("starting " + path)
						work := 60*time.Millisecond + time.Duration(len(path)%4)*35*time.Millisecond
						time.Sleep(work)
						logger.Info("processed item", "path", path, "result", "ok")
						st.Update("done " + path)
						return "processed:" + path, nil
					},
				}.Run(ctx)
				if err != nil {
					return err
				}
				logger.Info("map finished", "count", len(results), "first", results[0], "last", results[len(results)-1])
				fmt.Fprintf(cmd.ErrOrStderr(), "map collected %d results in order\n", len(results))
				return nil
			},
		})
	})
}
