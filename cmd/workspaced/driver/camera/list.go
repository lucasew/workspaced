package camera

import (
	"github.com/lucasew/workspaced/pkg/driver"
	cameraapi "github.com/lucasew/workspaced/pkg/driver/camera"

	"github.com/spf13/cobra"
)

func init() {
	Registry.Register(func(parent *cobra.Command) {
		parent.AddCommand(&cobra.Command{
			Use:   "list",
			Short: "List cameras",
			RunE: func(cmd *cobra.Command, args []string) error {
				drv, err := driver.Get[cameraapi.Driver](cmd.Context())
				if err != nil {
					return err
				}
				cams, err := drv.List(cmd.Context())
				if err != nil {
					return err
				}
				if len(cams) == 0 {
					return ErrNoCamerasFound
				}
				for _, cam := range cams {
					cmd.Printf("%s\t%s\n", cam.ID(), cam.Name())
				}
				return nil
			},
		})
	})
}
