package codebase

import (
	"workspaced/pkg/registry"

	"github.com/spf13/cobra"
)

var Registry registry.CommandRegistry

func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:                "codebase",
		Short:              "Tools for analyzing and managing codebases",
		DisableFlagParsing: true,
		SilenceUsage:       true,
	}
	return Registry.GetCommand(cmd)

}
