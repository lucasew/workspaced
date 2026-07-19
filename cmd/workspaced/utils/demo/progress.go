package demo

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/lucasew/workspaced/pkg/driver/notification"
	"github.com/lucasew/workspaced/pkg/logging"
)

func init() {
	Registry.Register(func(parent *cobra.Command) {
		parent.AddCommand(&cobra.Command{
			Use:   "progress",
			Short: "Demo progress notification",
			Run: func(cmd *cobra.Command, args []string) {
				ctx := cmd.Context()
				logger := logging.GetLogger(ctx)
				n := &notification.Notification{
					Title: "Progress Demo",
					Icon:  "utilities-terminal",
				}
				for i := 1; i <= 10; i++ {
					percent := i * 10
					n.Message = fmt.Sprintf("Step %d of 10...", i)
					n.HasProgress = true
					n.ID = 69
					n.Progress = float64(percent) / 100.0
					if err := notification.Notify(ctx, n); err != nil {
						logger.Error("error sending progress notification", "error", err)
					}
					time.Sleep(time.Second)
				}
				n.Message = "Demo complete!"
				n.Progress = 1.0
				if err := notification.Notify(ctx, n); err != nil {
					logger.Error("error sending final notification", "error", err)
				}
			},
		})
	})
}
