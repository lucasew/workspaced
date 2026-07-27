package nix

import (
	"github.com/lucasew/workspaced/internal/nix"

	"github.com/spf13/cobra"
)

func init() {
	Registry.Register(func(parent *cobra.Command) {
		cmd := &cobra.Command{
			Use:                "run <ref> [args...]",
			Short:              "Builds a package with RAM caching and runs it locally",
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

				resultPath, err := nix.Build(ctx, repo+"#"+item, true)
				if err != nil {
					return err
				}
				return runFromResultPath(ctx, resultPath, binary, runArgs)
			},
		}
		parent.AddCommand(cmd)
	})
}
