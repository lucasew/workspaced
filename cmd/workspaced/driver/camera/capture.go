package camera

import (
	"context"
	"errors"
	"fmt"
	"image"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/lewtec/lewkit/x/cmd"
	"github.com/lucasew/workspaced/internal/atomicfile"
	"github.com/lucasew/workspaced/pkg/driver"
	cameraapi "github.com/lucasew/workspaced/pkg/driver/camera"
)

var (
	ErrNoCamerasFound   = errors.New("no cameras found")
	ErrCameraNotFound   = errors.New("camera not found")
	ErrCaptureAllFailed = errors.New("capture from any camera")
)

type Capture struct {
	ID     cmd.StringArg `short:"i" long:"id" help:"camera ID or name (defaults to the first camera)"`
	Output cmd.StringArg `short:"o" long:"output" help:"output path (use - for stdout; defaults to cache directory)"`
}

func (Capture) Description() string { return "Capture a still frame" }

func (c *Capture) Run(ctx context.Context) error {
	return capture(ctx, os.Stdout, c.ID.Value(), c.Output.Value())
}

func capture(ctx context.Context, out io.Writer, id, outPath string) error {
	drv, err := driver.Get[cameraapi.Driver](ctx)
	if err != nil {
		return err
	}
	cams, err := drv.List(ctx)
	if err != nil {
		return err
	}
	cam, err := selectCamera(cams, id)
	if err != nil {
		return err
	}

	usedCam, img, err := captureFromCamera(ctx, cams, cam, id)
	if err != nil {
		return err
	}
	cam = usedCam

	if outPath == "-" {
		return png.Encode(out, img)
	}

	if outPath == "" {
		outPath, err = defaultCameraPath(cam)
		if err != nil {
			return err
		}
	}

	if err := os.MkdirAll(filepath.Dir(outPath), 0755); err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}
	if err := writePNGAtomic(outPath, img); err != nil {
		return err
	}
	fmt.Fprintln(out, outPath)
	return nil
}

func writePNGAtomic(outPath string, img image.Image) error {
	if err := atomicfile.WritePNG(outPath, img); err != nil {
		return fmt.Errorf("write camera capture: %w", err)
	}
	return nil
}

func selectCamera(cams []cameraapi.Camera, id string) (cameraapi.Camera, error) {
	if len(cams) == 0 {
		return nil, ErrNoCamerasFound
	}
	if id == "" {
		return cams[0], nil
	}
	for _, cam := range cams {
		if matchesCamera(cam, id) {
			return cam, nil
		}
	}
	return nil, fmt.Errorf("%w: %q", ErrCameraNotFound, id)
}

func captureFromCamera(ctx context.Context, cams []cameraapi.Camera, preferred cameraapi.Camera, id string) (cameraapi.Camera, image.Image, error) {
	if id != "" {
		img, err := preferred.Capture(ctx)
		return preferred, img, err
	}

	ordered := append([]cameraapi.Camera(nil), cams...)
	sort.SliceStable(ordered, func(i, j int) bool {
		return cameraPriority(ordered[i]) < cameraPriority(ordered[j])
	})

	var errs []string
	for _, cam := range ordered {
		img, err := cam.Capture(ctx)
		if err == nil {
			return cam, img, nil
		}
		errs = append(errs, fmt.Sprintf("%s: %v", cam.ID(), err))
	}
	if len(errs) == 0 {
		return nil, nil, ErrNoCamerasFound
	}
	return nil, nil, fmt.Errorf("%w: %s", ErrCaptureAllFailed, strings.Join(errs, "; "))
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
