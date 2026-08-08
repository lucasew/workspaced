package demo

import (
	"context"
	"fmt"
	"time"

	"workspaced/pkg/logging"
	"workspaced/pkg/taskgroup"

	"github.com/spf13/cobra"
)

const cpu10kItems = 10_000

func init() {
	Registry.Register(func(parent *cobra.Command) {
		parent.AddCommand(&cobra.Command{
			Use:   "cpu10k",
			Short: "Map.Run dispatches 10k × ~100ms CPU items; Control bar tracks children",
			Long: `taskgroup.Map.Run schedules one Control orchestrator (aggregate N/10000 progress)
and CPU-pool children (~100ms busy-wait each). Map owns progress — no outer
Control wrapper and no manual completion counter.

TERM=dumb → plain Wait.`,
			RunE: func(cmd *cobra.Command, args []string) error {
				ctx := cmd.Context()
				logger := logging.GetLogger(ctx)

				items := make([]int, cpu10kItems)
				for i := range items {
					items[i] = i
				}

				logger.Info("cpu10k: calling Map.Run",
					"items", len(items),
					"per_item", "100ms",
				)

				results, err := taskgroup.Map[int, uint64]{
					Name:     "cpu10k",
					Items:    items,
					PoolKind: taskgroup.CPU,
					TaskName: func(_ int, n int) string { return fmt.Sprintf("cpu:%d", n) },
					Fn: func(ctx context.Context, st *taskgroup.Status, n int) (uint64, error) {
						st.Update(fmt.Sprintf("item %d", n))
						deadline := time.Now().Add(100 * time.Millisecond)
						var h uint64 = uint64(n)*0x9e3779b97f4a7c15 + 1
						for time.Now().Before(deadline) {
							select {
							case <-ctx.Done():
								return 0, ctx.Err()
							default:
							}
							h ^= h << 13
							h ^= h >> 7
							h ^= h << 17
							h++
						}
						return h, nil
					},
				}.Run(ctx)
				if err != nil {
					return err
				}
				logger.Info("cpu10k finished",
					"results", len(results),
					"first", results[0],
					"last", results[len(results)-1],
				)
				return nil
			},
		})
	})
}
