package open

import (
	"github.com/spf13/cobra"
)

// miseCommand is a short alias for `open lazy --home mise`.
// Mise itself is not special-cased for install: it is a normal home lazy tool
// (registry:mise). This subcommand only keeps a convenient argv shape for
// shell scripts and the PATH shim (DisableFlagParsing for mise's own flags).
func miseCommand() *cobra.Command {
	return &cobra.Command{
		Use:                "mise [args...]",
		Short:              "Alias for: open lazy --home mise -- …",
		DisableFlagParsing: true,
		Long: `Convenience alias for the standard home lazy tool path:

  workspaced open lazy --home --bin mise mise -- [args...]

mise is declared in the home prelude as lazy_tools.mise (registry:mise) and
installed into the tool store like any other catalog tool. Package installs
via the mise: backend still shell out to that binary.

Examples:
  workspaced open mise version
  workspaced open mise install node@20
  workspaced open lazy --home mise -- version`,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runLazyTool(cmd.Context(), true, "mise", "mise", args)
		},
	}
}
