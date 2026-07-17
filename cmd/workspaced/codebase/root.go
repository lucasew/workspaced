package codebase

import (
	"workspaced/pkg/cmdregistry"

	"github.com/spf13/cobra"
)

var Registry cmdregistry.CommandRegistry

func GetCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:                "codebase",
		Short:              "Tools for analyzing and managing codebases",
		DisableFlagParsing: true,
		SilenceUsage:       true,
	}
	return Registry.FillCommands(cmd)
}
