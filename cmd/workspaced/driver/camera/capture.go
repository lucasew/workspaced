package camera

import (
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"time"

	"workspaced/pkg/driver"
	"workspaced/pkg/driver/camera"

	"github.com/spf13/cobra"
)

func init() {
	var output string
	var cameraID string

	Registry.Register(func(parent *cobra.Command) {
		cmd := &cobra.Command{
			Use:   "capture",
			Short: "Capture a still frame from camera",
			RunE: func(c *cobra.Command, args []string) error {
				d, err := driver.Get[camera.Driver](c.Context())
				if err != nil {
					return err
				}

				cams, err := d.List(c.Context())
				if err != nil {
					return err
				}
				if len(cams) == 0 {
					return fmt.Errorf("no camera devices found")
				}

				selected := cams[0]
				if cameraID != "" {
					found := false
					for _, cam := range cams {
						if cam.ID() == cameraID {
							selected = cam
							found = true
							break
						}
					}
					if !found {
						return fmt.Errorf("camera %q not found", cameraID)
					}
				}

				img, err := selected.Capture(c.Context())
				if err != nil {
					return err
				}

				target := output
				if target == "" {
					timestamp := time.Now().Format("2006-01-02_15-04-05")
					target = fmt.Sprintf("Camera_%s_%s.png", selected.ID(), timestamp)
				}

				if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
					return fmt.Errorf("failed to create output dir: %w", err)
				}

				f, err := os.Create(target)
				if err != nil {
					return fmt.Errorf("failed to create output file: %w", err)
				}
				defer f.Close()

				if err := png.Encode(f, img); err != nil {
					return fmt.Errorf("failed to encode image: %w", err)
				}

				c.Println(target)
				return nil
			},
		}
		cmd.Flags().StringVarP(&output, "output", "o", "", "Output image path (png)")
		cmd.Flags().StringVar(&cameraID, "id", "", "Camera device id")

		parent.AddCommand(cmd)
	})
}
