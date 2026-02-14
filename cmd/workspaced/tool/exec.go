package tool

import (
	"fmt"
	"github.com/spf13/cobra"
	"workspaced/pkg/tool"
)

func newExecCommand() *cobra.Command {
	return &cobra.Command{
		Use:                "exec <tool> [args...]",
		Short:              "Execute a tool",
		Hidden:             true,
		DisableFlagParsing: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return fmt.Errorf("tool name required")
			}
			toolName := args[0]
			toolArgs := args[1:]
			return tool.RunShim(cmd.Context(), toolName, toolArgs)
		},
	}
}
