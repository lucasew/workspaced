package svc

import (
	"context"
	"os"
	"strings"
	"syscall"

	"github.com/spf13/cobra"

	execdriver "github.com/lucasew/workspaced/pkg/driver/exec"
	"github.com/lucasew/workspaced/pkg/logging"
)

func init() {
	Registry.Register(func(parent *cobra.Command) {
		parent.AddCommand(&cobra.Command{
			Use:   "vncd",
			Short: "Start a VNC server (Wayland or X11)",
			RunE: func(cmd *cobra.Command, args []string) error {
				ctx := cmd.Context()
				waylandDisplay := os.Getenv("WAYLAND_DISPLAY")

				if waylandDisplay != "" {
					return runWaylandVNC(ctx)
				}
				return runXorgVNC(ctx)
			},
		})
	})
}

func runWaylandVNC(ctx context.Context) error {
	logger := logging.GetLogger(ctx)
	logger.Info("Starting wayvnc")
	host := os.Getenv("WAYVNC_HOST")
	if host == "" {
		tsIP, err := getTailscaleIP(ctx)
		if err == nil && tsIP != "" {
			host = tsIP
		} else {
			host = "127.0.0.1"
		}
	}

	bin, err := execdriver.Which(ctx, "wayvnc")
	if err != nil {
		return err
	}

	logger.Info("executing wayvnc", "host", host)
	return syscall.Exec(bin, []string{"wayvnc", host}, os.Environ())
}

func runXorgVNC(ctx context.Context) error {
	logger := logging.GetLogger(ctx)
	logger.Info("Starting x0vncserver")
	bin, err := execdriver.Which(ctx, "x0vncserver")
	if err != nil {
		return err
	}

	args := []string{
		"x0vncserver",
		"-display=:0",
		"-SecurityTypes", "None",
		"-ImprovedHextile=1",
		"-RawKeyboard=1",
	}

	logger.Info("executing x0vncserver")
	return syscall.Exec(bin, args, os.Environ())
}

func getTailscaleIP(ctx context.Context) (string, error) {
	out, err := execdriver.MustRun(ctx, "tailscale", "ip", "-4").Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}
