package codebase

import (
	"github.com/spf13/cobra"
)

func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:                "codebase",
		Short:              "Tools for analyzing and managing codebases",
		DisableFlagParsing: true,
		SilenceUsage:       true,
	}

	cmd.AddCommand(newLintCommand())
	cmd.AddCommand(newFormatCommand())
	cmd.AddCommand(newCiStatusCommand())

	return cmd
}
