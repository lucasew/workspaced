package demo

import (
	"context"
	"fmt"
	"time"

	"github.com/lucasew/workspaced/pkg/driver/notification"
	"github.com/lucasew/workspaced/pkg/logging"
)

type Progress struct{}

func (Progress) Description() string { return "Demo progress notification" }

func (*Progress) Run(ctx context.Context) error {
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
	return nil
}
