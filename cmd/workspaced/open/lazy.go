package open

import (
	"fmt"
	"os"
	execdriver "workspaced/pkg/driver/exec"
	"workspaced/pkg/tool"
	_ "workspaced/pkg/tool/prelude"

	"github.com/spf13/cobra"
)

func lazyCommand() *cobra.Command {
	var binName string
	var homeMode bool

	cmd := &cobra.Command{
		Use:   "lazy <tool-name> [args...]",
		Short: "Run a lazy tool resolved from home config and workspaced.lock.json",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			toolName := args[0]
			toolArgs := args[1:]
			if len(toolArgs) > 0 && toolArgs[0] == "--" {
				toolArgs = toolArgs[1:]
			}
			if binName == "" {
				binName = toolName
			}

			resolver := tool.ResolveLazyTool
			if homeMode {
				resolver = tool.ResolveHomeLazyTool
			}

			binPath, err := resolver(cmd.Context(), toolName, binName)
			if err != nil {
				return err
			}

			command, err := execdriver.Run(cmd.Context(), binPath, toolArgs...)
			if err != nil {
				return fmt.Errorf("failed to create command: %w", err)
			}
			command.Stdin = os.Stdin
			command.Stdout = os.Stdout
			command.Stderr = os.Stderr
			return command.Run()
		},
	}

	cmd.Flags().StringVar(&binName, "bin", "", "Binary name to resolve inside the tool package")
	cmd.Flags().BoolVar(&homeMode, "home", false, "Resolve the lazy tool using the home/dotfiles workspace")
	cmd.Flags().SetInterspersed(false)

	return cmd
}
