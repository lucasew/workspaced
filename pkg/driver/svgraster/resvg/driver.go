package resvg

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/lucasew/workspaced/internal/tool"
	"github.com/lucasew/workspaced/pkg/driver"
	"github.com/lucasew/workspaced/pkg/driver/exec"
	"github.com/lucasew/workspaced/pkg/driver/svgraster"
	"github.com/lucasew/workspaced/pkg/logging"
)

const defaultResvgSpec = "registry:resvg"

type Driver struct{}

var (
	resvgOnce sync.Once
	resvgPath string
	resvgErr  error
)

// resolveResvg ensures the resvg binary is resolved/installed exactly once
// (even across many Driver instances created by driver.Get). This prevents
// repeated "latest" version lookups against GitHub (via registry:resvg) on
// every RasterizeSVG call during bulk icon processing.
func resolveResvg(ctx context.Context) (string, error) {
	resvgOnce.Do(func() {
		m, err := tool.NewManager()
		if err != nil {
			resvgErr = fmt.Errorf("failed to create tool manager: %w", err)
			return
		}
		bin, err := m.EnsureInstalled(ctx, defaultResvgSpec, "resvg")
		if err != nil {
			resvgErr = fmt.Errorf("failed to resolve resvg via tool (%s): %w", defaultResvgSpec, err)
			return
		}
		// Verify on first resolution (cheap --version) so Ensure and first
		// use behave the same as before.
		// exec.Run pre-sets Stderr (for Output/Run); do not use CombinedOutput.
		c, err := exec.Run(ctx, bin, "--version")
		if err != nil {
			resvgErr = fmt.Errorf("failed to prepare resvg command: %w", err)
			return
		}
		if err := c.Run(); err != nil {
			resvgErr = fmt.Errorf("resvg --version check failed: %w", err)
			return
		}
		resvgPath = bin
	})
	if resvgErr != nil {
		return "", resvgErr
	}
	return resvgPath, nil
}

func (d *Driver) Ensure(ctx context.Context) error {
	_, err := resolveResvg(ctx)
	return err
}

func (d *Driver) RasterizeSVG(ctx context.Context, svg string, width int, height int) (image.Image, error) {
	bin, err := resolveResvg(ctx)
	if err != nil {
		return nil, err
	}

	tmpDir, err := os.MkdirTemp("", "workspaced-svgraster-*")
	if err != nil {
		return nil, err
	}
	defer logging.RunCleanup(ctx, "remove_all", func() error { return os.RemoveAll(tmpDir) })

	inSVG := filepath.Join(tmpDir, "input.svg")
	outPNG := filepath.Join(tmpDir, "output.png")
	if err := os.WriteFile(inSVG, []byte(svg), 0600); err != nil {
		return nil, err
	}

	c, err := exec.Run(
		ctx,
		bin,
		"--width", fmt.Sprintf("%d", width),
		"--height", fmt.Sprintf("%d", height),
		inSVG,
		outPNG,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare resvg command: %w", err)
	}
	// Capture diagnostics without CombinedOutput (Stderr already attached).
	var stderr bytes.Buffer
	c.Stderr = &stderr
	if err := c.Run(); err != nil {
		return nil, fmt.Errorf("resvg failed: %w: %s", err, strings.TrimSpace(stderr.String()))
	}

	f, err := os.Open(outPNG)
	if err != nil {
		return nil, err
	}
	defer logging.Close(ctx, f)

	img, err := png.Decode(f)
	if err != nil {
		return nil, err
	}
	return img, nil
}

type Factory struct{}

func (f Factory) ID() string { return "resvg" }
func (f Factory) Name() string {
	return "resvg"
}
func (f Factory) CheckCompatibility(ctx context.Context) error {
	// resvg is installed on demand via the tool registry (catalog).
	return nil
}
func (f Factory) New(ctx context.Context) (svgraster.Driver, error) {
	return &Driver{}, nil
}

func init() {
	driver.Register[svgraster.Driver](Factory{})
}
