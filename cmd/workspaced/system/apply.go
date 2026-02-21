package system

import (
	"context"
	"fmt"
	"workspaced/pkg/env"
	"workspaced/pkg/logging"
	"workspaced/pkg/nix"

	"github.com/spf13/cobra"
)

func RunApply(ctx context.Context, action string, dryRun bool) error {
	logger := logging.GetLogger(ctx)

	if !env.IsNixOS() {
		logger.Info("not running on NixOS; skipping system apply")
		return nil
	}

	logger.Info("running NixOS rebuild", "action", action)
	if dryRun {
		logger.Info("dry-run: skipping nixos-rebuild")
		return nil
	}

	flake := ""
	hostname := env.GetHostname()
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
			dryRun, _ := cmd.Flags().GetBool("dry-run")
			return RunApply(cmd.Context(), action, dryRun)
		},
	}

	cmd.Flags().BoolP("dry-run", "d", false, "Only show what would be done")
	return cmd
}
