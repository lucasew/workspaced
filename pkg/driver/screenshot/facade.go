package screenshot

import (
	"context"
	"fmt"
	"image"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/lucasew/workspaced/internal/atomicfile"
	"github.com/lucasew/workspaced/internal/configcue"
	"github.com/lucasew/workspaced/pkg/driver"
	"github.com/lucasew/workspaced/pkg/driver/clipboard"
	"github.com/lucasew/workspaced/pkg/driver/notification"
	"github.com/lucasew/workspaced/pkg/driver/wm"
	"github.com/lucasew/workspaced/pkg/logging"
)

func ResolveRect(ctx context.Context, targetType TargetType) (*wm.Rect, error) {
	switch targetType {
	case TargetAll:
		return nil, nil
	case TargetOutput:
		_, rect, err := wm.GetFocusedOutput(ctx)
		return rect, err
	case TargetWindow:
		return wm.GetFocusedWindowRect(ctx)
	case TargetSelection:
		d, err := driver.Get[Driver](ctx)
		if err != nil {
			return nil, err
		}
		return d.SelectArea(ctx)
	default:
		return nil, fmt.Errorf("unknown target type: %v", targetType)
	}
}

func Capture(ctx context.Context, targetType TargetType) (string, error) {
	d, err := driver.Get[Driver](ctx)
	if err != nil {
		return "", err
	}

	rect, err := ResolveRect(ctx, targetType)
	if err != nil {
		return "", err
	}

	img, err := d.Capture(ctx, rect)
	if err != nil {
		return "", err
	}

	cfg, err := configcue.LoadHome(ctx)
	if err != nil {
		return "", err
	}
	var screenshot struct {
		Dir string `json:"dir"`
	}
	if err := cfg.Decode("screenshot", &screenshot); err != nil {
		return "", err
	}

	dir := strings.TrimSpace(screenshot.Dir)
	if dir == "" {
		return "", fmt.Errorf("screenshot dir not configured")
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("create screenshot dir: %w", err)
	}

	timestamp := time.Now().Format("2006-01-02_15-04-05")
	filename := fmt.Sprintf("Screenshot_%s.png", timestamp)
	path := filepath.Join(dir, filename)

	if err := writePNGAtomic(path, img); err != nil {
		return "", err
	}

	// Post-processing: Clipboard
	go func() {
		if err := clipboard.WriteImage(ctx, img); err != nil {
			logging.ReportError(ctx, err)
		}
	}()

	// Post-processing: Notification
	notifySaved(ctx, path, targetType)

	return path, nil
}

func notifySaved(ctx context.Context, path string, target TargetType) {
	strategy := "Unknown"
	switch target {
	case TargetAll:
		strategy = "All screens"
	case TargetOutput:
		strategy = "Current monitor"
	case TargetWindow:
		strategy = "Current window"
	case TargetSelection:
		strategy = "Selected area"
	}

	n := notification.Notification{
		Title:   fmt.Sprintf("Screenshot Saved (%s)", strategy),
		Message: path,
		Icon:    "camera-photo",
	}
	if err := notification.Notify(ctx, &n); err != nil {
		logging.ReportError(ctx, err)
	}
}

func writePNGAtomic(path string, img image.Image) error {
	if err := atomicfile.WritePNG(path, img); err != nil {
		return fmt.Errorf("write screenshot: %w", err)
	}
	return nil
}
