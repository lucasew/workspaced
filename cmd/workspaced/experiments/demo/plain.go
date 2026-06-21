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
