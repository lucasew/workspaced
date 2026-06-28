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
			Short: "Map dispatches 10k × ~100ms CPU items; Control bar tracks children",
			Long: `A thin Control holder keeps the root group alive for RunBubbleTea while
taskgroup.Map schedules its own Control orchestrator (aggregate N/10000 progress)
and CPU-pool children (~100ms busy-wait each). Map owns progress — no manual
completion counter.

TERM=dumb → plain Wait.`,
			RunE: func(cmd *cobra.Command, args []string) error {
				g := taskgroup.MustFromContext(cmd.Context())

				items := make([]int, cpu10kItems)
				for i := range items {
					items[i] = i
				}

				// Holder so RunBubbleTea has work scheduled before Wait; Map creates
				// the real Control progress task on this group.
				g.Go("cpu10k", taskgroup.Control, func(ctx context.Context, _ *taskgroup.Status) error {
					logger := logging.GetLogger(ctx)
					logger.Info("cpu10k: calling Map (Control progress over CPU children)",
						"items", len(items),
						"per_item", "100ms",
					)

					results, err := taskgroup.Map(ctx, func(int) taskgroup.PoolKind { return taskgroup.CPU }, items,
						func(_ int, n int) string {
							return fmt.Sprintf("cpu:%d", n)
						},
						func(ctx context.Context, st *taskgroup.Status, n int) (uint64, error) {
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
					)
					if err != nil {
						return err
					}
					logger.Info("cpu10k finished",
						"results", len(results),
						"first", results[0],
						"last", results[len(results)-1],
					)
					return nil
				})

				return g.RunBubbleTea()
			},
		})
	})
}
