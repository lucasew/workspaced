package input

import (
	"context"

	"github.com/lucasew/workspaced/pkg/driver"
	"github.com/lucasew/workspaced/pkg/driver/dialog"

	"github.com/spf13/cobra"
)

func init() {
	Registry.Register(func(parent *cobra.Command) {
		parent.AddCommand(dialogCmd("launch", "Application launcher", func(d dialog.Driver, ctx context.Context) error {
			return d.RunApp(ctx)
		}))
		parent.AddCommand(dialogCmd("window", "Window switcher", func(d dialog.Driver, ctx context.Context) error {
			return d.SwitchWindow(ctx)
		}))
	})
}

func dialogCmd(use, short string, run func(dialog.Driver, context.Context) error) *cobra.Command {
	return &cobra.Command{
		Use:   use,
		Short: short,
		RunE: func(c *cobra.Command, args []string) error {
			d, err := driver.Get[dialog.Driver](c.Context())
			if err != nil {
				return err
			}
			return run(d, c.Context())
		},
	}
}
