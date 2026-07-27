package screenshot

import (
	"errors"

	dapi "github.com/lucasew/workspaced/pkg/api"
	"github.com/lucasew/workspaced/pkg/driver/screenshot"

	"github.com/spf13/cobra"
)

func init() {
	type captureCmd struct {
		use, short string
		target     screenshot.TargetType
	}
	cmds := []captureCmd{
		{"all", "Capture all outputs", screenshot.TargetAll},
		{"full", "Capture full screen (all outputs)", screenshot.TargetAll},
		{"output", "Capture current output (monitor)", screenshot.TargetOutput},
		{"window", "Capture current window", screenshot.TargetWindow},
	}
	Registry.Register(func(parent *cobra.Command) {
		for _, cc := range cmds {
			cc := cc
			parent.AddCommand(&cobra.Command{
				Use:   cc.use,
				Short: cc.short,
				RunE: func(c *cobra.Command, args []string) error {
					path, err := screenshot.Capture(c.Context(), cc.target)
					if err != nil {
						return err
					}
					c.Println(path)
					return nil
				},
			})
		}
		parent.AddCommand(&cobra.Command{
			Use:   "select",
			Short: "Capture selected area",
			RunE: func(c *cobra.Command, args []string) error {
				path, err := screenshot.Capture(c.Context(), screenshot.TargetSelection)
				if err != nil {
					if errors.Is(err, dapi.ErrCanceled) {
						return nil
					}
					return err
				}
				if path != "" {
					c.Println(path)
				}
				return nil
			},
		})
	})
}
