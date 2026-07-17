package svc

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"workspaced/pkg/driver"
	"workspaced/pkg/driver/battery"
	"workspaced/pkg/logging"
)

func init() {
	Registry.Register(func(parent *cobra.Command) {
		parent.AddCommand(&cobra.Command{
			Use:   "osmardetector",
			Short: "Annoying beep each second if laptop stops charging",
			Run: func(cmd *cobra.Command, args []string) {
				ctx := cmd.Context()
				ticker := time.NewTicker(1 * time.Second)
				defer ticker.Stop()

				logger := logging.GetLogger(ctx)
				logger.Info("osmardetector started")
				driver, err := driver.Get[battery.Driver](ctx)
				if err != nil {
					logger := logging.GetLogger(ctx)
					logger.Error("failed to get battery driver", "error", err)
					return
				}

				for {
					select {
					case <-ctx.Done():
						return
					case <-ticker.C:
						status, err := driver.BatteryStatus(ctx)
						if err != nil {
							logger := logging.GetLogger(ctx)
							logger.Error("failed to get battery status", "error", err)
							continue
						}
						if status == battery.Discharging {
							fmt.Print("\aAi!")
						}
					}
				}
			},
		})
	})
}
