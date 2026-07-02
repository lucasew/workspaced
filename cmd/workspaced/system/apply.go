package system

import (
	"context"
	"fmt"
	"workspaced/pkg/cmdctx"
	"workspaced/pkg/configcue"
	envdriver "workspaced/pkg/driver/env"
	"workspaced/pkg/logging"
	"workspaced/pkg/modfile"
	_ "workspaced/pkg/modfile/sourceprovider/prelude"
	"workspaced/pkg/nix"
	"workspaced/pkg/tool"

	"github.com/spf13/cobra"
)

func RunApply(ctx context.Context, action string) error {
	logger := logging.GetLogger(ctx)
	dryRun := cmdctx.IsDryRun(ctx)

	dotfilesRoot, err := envdriver.GetDotfilesRoot(ctx)
	if err != nil {
		return fmt.Errorf("failed to get dotfiles root: %w", err)
	}
	cfg, err := configcue.LoadHome(ctx)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}
	if _, err := tool.RefreshWorkspaceLocks(ctx, modfile.NewWorkspace(dotfilesRoot), cfg); err != nil {
		return fmt.Errorf("failed to refresh workspace lockfile: %w", err)
	}

	if !envdriver.IsNixOS(ctx) {
		logger.Info("not running on NixOS; skipping system apply")
		return nil
	}

	logger.Info("running NixOS rebuild", "action", action)
	if dryRun {
		logger.Info("dry-run: skipping nixos-rebuild")
		return nil
	}

	flake := ""
	hostname, err := envdriver.GetHostname(ctx)
	if err != nil {
		return fmt.Errorf("hostname: %w", err)
	}
	if hostname == "riverwood" {
		logger.Info("performing remote build for riverwood")
		ref := fmt.Sprintf(".#nixosConfigurations.%s.config.system.build.toplevel", hostname)
		nixResult, err := nix.RemoteBuild(ctx, ref, "whiterun", true)
		if err != nil {
			return fmt.Errorf("remote build failed: %w", err)
		}
		flake = nixResult
	}

	return nix.Rebuild(ctx, action, flake)
}

func getApplyCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "apply [action]",
		Short: "Apply system-level configuration (NixOS rebuild)",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			action := "switch"
			if len(args) > 0 {
				action = args[0]
			}
			return RunApply(cmd.Context(), action)
		},
	}
	return cmd
}
