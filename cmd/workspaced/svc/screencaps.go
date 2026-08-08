package svc

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"workspaced/pkg/driver/screen"
	"workspaced/pkg/logging"
)

func init() {
	Registry.Register(func(parent *cobra.Command) {
		parent.AddCommand(&cobra.Command{
			Use:   "screencaps",
			Short: "Monitor CapsLock and toggle screen DPMS",
			Run: func(cmd *cobra.Command, args []string) {
				monitorCapsLock(cmd.Context())
			},
		})
	})
}

func monitorCapsLock(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	matches, _ := filepath.Glob("/sys/class/leds/*capslock/brightness")
	if len(matches) == 0 {
		logger := logging.GetLogger(ctx)
		logger.Warn("no capslock leds found")
		return
	}

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			capsActive := false
			for _, m := range matches {
				data, err := os.ReadFile(m)
				if err == nil && strings.TrimSpace(string(data)) == "1" {
					capsActive = true
					break
				}
			}

			logger := logging.GetLogger(ctx)
			screenActive, err := screen.IsDPMSOn(ctx)
			if err != nil {
				logger.Error("on checking if screen is active", "error", err)
			}
			if !capsActive != screenActive {
				logger.Info("toggling screen", "active", !capsActive)
				_ = screen.SetDPMS(ctx, !capsActive)
			}
		}
	}
}
