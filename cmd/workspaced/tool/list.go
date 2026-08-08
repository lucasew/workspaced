package tool

import (
	"fmt"
	"workspaced/pkg/tool"

	"github.com/spf13/cobra"
)

func init() {
	Registry.FromGetter(
		func() *cobra.Command {
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
		})
}
