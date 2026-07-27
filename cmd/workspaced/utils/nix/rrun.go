package nix

import (
	"github.com/lucasew/workspaced/internal/nix"

	"github.com/spf13/cobra"
)

func init() {
	Registry.Register(func(parent *cobra.Command) {
		var target string
		cmd := &cobra.Command{
			Use:                "rrun <ref> [args...]",
			Short:              "Builds a package remotely and runs it locally",
			Args:               cobra.MinimumNArgs(1),
			DisableFlagParsing: true,
			RunE: func(cmd *cobra.Command, args []string) error {
				if len(args) == 0 {
					return ErrNoFlakeRef
				}
				ctx := cmd.Context()
				ref := args[0]
				runArgs := stripLeadingDashArgs(args[1:])
				repo, item, binary := parseFlakeRef(ref)

				resultPath, err := nix.RemoteBuild(ctx, repo+"#"+item, target, true)
				if err != nil {
					return err
				}
				return runFromResultPath(ctx, resultPath, binary, runArgs)
			},
		}
		parent.AddCommand(cmd)
	})
}
