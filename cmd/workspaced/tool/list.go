package tool

import (
	"fmt"
	"github.com/spf13/cobra"
	"workspaced/pkg/tool"
)

func newListCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List installed tools",
		RunE: func(cmd *cobra.Command, args []string) error {
			manager, err := tool.NewManager()
			if err != nil {
				return err
			}
			tools, err := manager.ListInstalled()
			if err != nil {
				return err
			}
			for _, t := range tools {
				fmt.Printf("%s %s\n", t.Name, t.Version)
			}
			return nil
		},
	}
}
