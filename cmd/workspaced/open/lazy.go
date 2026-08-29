package open

import (
	"context"
	"fmt"
	"os"
	"os/exec"

	"github.com/lucasew/workspaced/internal/tool"
	_ "github.com/lucasew/workspaced/internal/tool/prelude"
	execdriver "github.com/lucasew/workspaced/pkg/driver/exec"
	"github.com/lucasew/workspaced/pkg/taskgroup"

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

			ctx := cmd.Context()
			g := taskgroup.MustFromContext(ctx)
			var theCmd *exec.Cmd
			g.Go("open:lazy:"+toolName, taskgroup.Control, func(ctx context.Context, s *taskgroup.Status) error {
				s.Update("resolving " + toolName)
				binPath, err := resolver(ctx, toolName, binName)
				if err != nil {
					return err
				}
				// Detach so session teardown does not cancel the child.
				execCtx := context.WithoutCancel(ctx)
				c, err := execdriver.Run(execCtx, binPath, toolArgs...)
				if err != nil {
					return fmt.Errorf("create command: %w", err)
				}
				theCmd = c
				return nil
			})
			taskgroup.MustSessionFrom(ctx).AfterWait(func() error {
				if theCmd == nil {
					return nil
				}
				theCmd.Stdin = os.Stdin
				theCmd.Stdout = os.Stdout
				theCmd.Stderr = os.Stderr
				return theCmd.Run()
			})
			return nil
		},
	}

	cmd.Flags().StringVar(&binName, "bin", "", "Binary name to resolve inside the tool package")
	cmd.Flags().BoolVar(&homeMode, "home", false, "Resolve the lazy tool using the home/dotfiles workspace")
	cmd.Flags().SetInterspersed(false)

	return cmd
}
