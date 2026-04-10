package camera

import (
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"workspaced/pkg/driver"
	cameraapi "workspaced/pkg/driver/camera"

	"github.com/spf13/cobra"
)

func init() {
	Registry.Register(func(parent *cobra.Command) {
		cmd := &cobra.Command{
			Use:   "capture",
			Short: "Capture a still frame",
			RunE: func(cmd *cobra.Command, args []string) error {
				id, _ := cmd.Flags().GetString("id")
				outPath, _ := cmd.Flags().GetString("output")
				return capture(cmd, id, outPath)
			},
		}
		cmd.Flags().StringP("id", "i", "", "camera ID or name (defaults to the first camera)")
		cmd.Flags().StringP("output", "o", "", "output path (use - for stdout; defaults to cache directory)")
		parent.AddCommand(cmd)
	})
}

func capture(cmd *cobra.Command, id, outPath string) error {
	drv, err := driver.Get[cameraapi.Driver](cmd.Context())
	if err != nil {
		return err
	}
	cams, err := drv.List(cmd.Context())
	if err != nil {
		return err
	}
	cam, err := selectCamera(cams, id)
	if err != nil {
		return err
	}

	usedCam, img, err := captureFromCamera(cmd, cams, cam, id)
	if err != nil {
		return err
	}
	cam = usedCam

	if outPath == "-" {
		return png.Encode(cmd.OutOrStdout(), img)
	}

	if outPath == "" {
		outPath, err = defaultCameraPath(cam)
		if err != nil {
			return err
		}
	}

	if err := os.MkdirAll(filepath.Dir(outPath), 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}
	out, err := os.Create(outPath)
	if err != nil {
		return err
	}
	defer func() {
		_ = out.Close()
	}()
	if err := png.Encode(out, img); err != nil {
		return err
	}
	cmd.Println(outPath)
	return nil
}

func selectCamera(cams []cameraapi.Camera, id string) (cameraapi.Camera, error) {
	if len(cams) == 0 {
		return nil, fmt.Errorf("no cameras found")
	}
	if id == "" {
		return cams[0], nil
	}
	for _, cam := range cams {
		if matchesCamera(cam, id) {
			return cam, nil
		}
	}
	return nil, fmt.Errorf("camera %q not found", id)
}

func captureFromCamera(cmd *cobra.Command, cams []cameraapi.Camera, preferred cameraapi.Camera, id string) (cameraapi.Camera, image.Image, error) {
	if id != "" {
		img, err := preferred.Capture(cmd.Context())
		return preferred, img, err
	}

	ordered := append([]cameraapi.Camera(nil), cams...)
	sort.SliceStable(ordered, func(i, j int) bool {
		return cameraPriority(ordered[i]) < cameraPriority(ordered[j])
	})

	var errs []string
	for _, cam := range ordered {
		img, err := cam.Capture(cmd.Context())
		if err == nil {
			return cam, img, nil
		}
		errs = append(errs, fmt.Sprintf("%s: %v", cam.ID(), err))
	}
	if len(errs) == 0 {
		return nil, nil, fmt.Errorf("no cameras found")
	}
	return nil, nil, fmt.Errorf("failed to capture from any camera: %s", strings.Join(errs, "; "))
}

func cameraPriority(cam cameraapi.Camera) int {
	name := strings.ToLower(cam.Name())
	if strings.Contains(name, "dummy") || strings.Contains(name, "virtual") {
		return 1
	}
	return 0
}

func matchesCamera(cam cameraapi.Camera, id string) bool {
	if cam.ID() == id || filepath.Base(cam.ID()) == id {
		return true
	}
	if cam.Name() == id {
		return true
	}
	return sanitizeComponent(cam.Name()) == sanitizeComponent(id)
}

func defaultCameraPath(cam cameraapi.Camera) (string, error) {
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		home, homeErr := os.UserHomeDir()
		if homeErr != nil {
			return "", err
		}
		cacheDir = filepath.Join(home, ".cache")
	}
	cameraDir := filepath.Join(cacheDir, "workspaced", "camera")
	if err := os.MkdirAll(cameraDir, 0755); err != nil {
		return "", err
	}
	stamp := time.Now().Format("2006-01-02_15-04-05")
	name := sanitizeComponent(cam.Name())
	if name == "" {
		name = sanitizeComponent(filepath.Base(cam.ID()))
	}
	if name == "" {
		name = "camera"
	}
	return filepath.Join(cameraDir, fmt.Sprintf("%s_%s.png", stamp, name)), nil
}

func sanitizeComponent(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
		case r >= 'A' && r <= 'Z':
			b.WriteRune(r)
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '-' || r == '_' || r == '.':
			b.WriteRune(r)
		default:
			b.WriteRune('_')
		}
	}
	return strings.Trim(b.String(), "_")
}
