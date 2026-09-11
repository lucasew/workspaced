package nix

import (
	"context"
	"fmt"
	"strings"

	"github.com/lucasew/workspaced/internal/executil"
	"github.com/lucasew/workspaced/internal/nix"
	envdriver "github.com/lucasew/workspaced/pkg/driver/env"
	execdriver "github.com/lucasew/workspaced/pkg/driver/exec"
	"github.com/lucasew/workspaced/pkg/driver/notification"
	"github.com/lucasew/workspaced/pkg/logging"
	"github.com/lucasew/workspaced/pkg/taskgroup"

	"github.com/lewtec/lewkit/x/cmd"
)

type Deploy struct {
	Flake  cmd.StringArg `short:"f" long:"flake" help:"Flake reference to use"`
	Action cmd.StringArg `short:"a" long:"action" help:"Action to perform (switch, boot, test). If empty, auto-detects."`
	nodes  []cmd.StringArg
}

func (Deploy) Description() string {
	return "Deploy NixOS and Home Manager configurations to remote nodes"
}

func (d *Deploy) Run(ctx context.Context) error {
	nodes := cmd.Values(d.nodes)
	if len(nodes) == 0 {
		nodes = []string{"riverwood", "whiterun"}
	}

	flake := d.Flake.Value()
	if flake == "" {
		root, err := envdriver.GetDotfilesRoot(ctx)
		if err != nil {
			return err
		}
		flake = root
	}

	action := d.Action.Value()

	err := taskgroup.Each[string]{
		Name:     "nix-deploy",
		Items:    nodes,
		PoolKind: taskgroup.Internet,
		TaskName: func(_ int, node string) string { return "deploy:" + node },
		Fn: func(ctx context.Context, s *taskgroup.Status, node string) error {
			s.Update(node)
			logger := logging.GetLogger(ctx).With("node", node)
			logger.Info("Deploying to node")
			if err := deployNode(ctx, flake, node, action); err != nil {
				logger.Error("Failed to deploy to node", "error", err)
				return err
			}
			return nil
		},
	}.Run(ctx)
	if err != nil {
		return err
	}

	deployed := append([]string(nil), nodes...)
	taskgroup.MustSessionFrom(ctx).AfterWait(func() error {
		n := notification.Notification{
			Title:   "NixOS Deploy",
			Message: fmt.Sprintf("Deploy completed for: %s", strings.Join(deployed, ", ")),
			Icon:    "nix-snowflake",
		}
		if err := notification.Notify(ctx, &n); err != nil {
			logging.GetLogger(ctx).Error("failed to send notification", "error", err)
		}
		return nil
	})
	return nil
}

func deployNode(ctx context.Context, flake, node, action string) error {
	logger := logging.GetLogger(ctx)
	logger = logger.With("node", node)
	// 1. Build outputs
	logger.Info("Building configuration for node")
	toplevelPath := fmt.Sprintf("nixosConfigurations.%s.config.system.build.toplevel", node)
	toplevel, err := nix.GetFlakeOutput(ctx, flake, toplevelPath)
	if err != nil {
		return fmt.Errorf("build toplevel for %s: %w", node, err)
	}

	// 2. Copy closures
	logger.Info("Copying closures to node")
	if err := nix.CopyClosure(ctx, node, toplevel, nix.To); err != nil {
		return fmt.Errorf("copy toplevel to %s: %w", node, err)
	}

	// 3. Auto-detect action if not specified
	if action == "" {
		action = "boot"
		localUsedOut, err := execdriver.MustRun(ctx, "realpath", fmt.Sprintf("%s/etc/.nixpkgs-used", toplevel)).Output()
		if err == nil {
			localUsed := strings.TrimSpace(string(localUsedOut))
			remoteUsedOut, err := execdriver.MustRun(ctx, "ssh", node, "realpath /etc/.nixpkgs-used").Output()
			if err == nil {
				remoteUsed := strings.TrimSpace(string(remoteUsedOut))
				if localUsed == remoteUsed {
					action = "switch"
				}
			}
		}
	}

	// 4. Switch system configuration
	logger.Info("Switching system configuration on node", "action", action)
	currentSystemOut, err := execdriver.MustRun(ctx, "ssh", node, "realpath /run/current-system").Output()
	if err == nil {
		currentSystem := strings.TrimSpace(string(currentSystemOut))
		if currentSystem == toplevel {
			logger.Info("Node already running the same configuration")
			return nil
		}
	}

	switchCmdArgs := []string{"ssh", "-t", node, "pkexec", fmt.Sprintf("%s/bin/switch-to-configuration", toplevel), action}
	cmdSwitch := execdriver.MustRun(ctx, switchCmdArgs[0], switchCmdArgs[1:]...)
	executil.InheritContextWriters(ctx, cmdSwitch)
	if err := cmdSwitch.Run(); err != nil {
		return fmt.Errorf("switch configuration on %s: %w", node, err)
	}

	return nil
}
