package camera

import (
	"fmt"

	"workspaced/pkg/driver"
	"workspaced/pkg/driver/camera"

	"github.com/spf13/cobra"
)

func init() {
	Registry.Register(func(parent *cobra.Command) {
		parent.AddCommand(&cobra.Command{
			Use:   "list",
			Short: "List available cameras",
			RunE: func(c *cobra.Command, args []string) error {
				d, err := driver.Get[camera.Driver](c.Context())
				if err != nil {
					return err
				}

				cams, err := d.List(c.Context())
				if err != nil {
					return err
				}

				for _, cam := range cams {
					c.Println(fmt.Sprintf("%s\t%s", cam.ID(), cam.Name()))
				}

				return nil
			},
		})
	})
}
