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
				return nil
			},
		})
	})
}
