package svc

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/lucasew/workspaced/pkg/driver/screen"
	"github.com/lucasew/workspaced/pkg/logging"
)

type Screencaps struct{}

func (Screencaps) Description() string {
	return "Monitor CapsLock and toggle screen DPMS"
}

func (*Screencaps) Run(ctx context.Context) error {
	monitorCapsLock(ctx)
	return nil
}

func monitorCapsLock(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	matches, err := filepath.Glob("/sys/class/leds/*capslock/brightness")
	if err != nil || len(matches) == 0 {
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
				if err := screen.SetDPMS(ctx, !capsActive); err != nil {
					logger.Error("failed to set screen DPMS", "active", !capsActive, "error", err)
				}
			}
		}
	}
}
