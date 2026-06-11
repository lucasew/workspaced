package resvg

import (
	"context"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"

	"workspaced/pkg/driver"
	"workspaced/pkg/driver/svgraster"
	"workspaced/pkg/logging"
	"workspaced/pkg/tool"
)

const defaultResvgSpec = "registry:resvg"

type Driver struct{}

func (d *Driver) Ensure(ctx context.Context) error {
	// Calling EnsureAndRun triggers tool resolution/install via the
	// configured backend (e.g. downloading the registry:resvg release
	// if not present). We use a cheap --version invocation.
	// The actual rasterization calls later will be fast.
	c, err := tool.EnsureAndRun(ctx, defaultResvgSpec, "resvg", "--version")
	if err != nil {
		return fmt.Errorf("failed to resolve resvg via tool (%s): %w", defaultResvgSpec, err)
	}
	// We don't strictly need the output for Ensure, but running it
	// verifies the binary is executable (consistent with RasterizeSVG).
	if _, err := c.CombinedOutput(); err != nil {
		return fmt.Errorf("resvg --version check failed: %w", err)
	}
	return nil
}

func (d *Driver) RasterizeSVG(ctx context.Context, svg string, width int, height int) (image.Image, error) {
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

	c, err := tool.EnsureAndRun(
		ctx,
		defaultResvgSpec,
		"resvg",
		"--width", fmt.Sprintf("%d", width),
		"--height", fmt.Sprintf("%d", height),
		inSVG,
		outPNG,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve resvg via tool (%s): %w", defaultResvgSpec, err)
	}
	if out, err := c.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("resvg failed: %w: %s", err, strings.TrimSpace(string(out)))
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

type Provider struct{}

func (p Provider) ID() string { return "resvg" }
func (p Provider) Name() string {
	return "resvg"
}
func (p Provider) CheckCompatibility(ctx context.Context) error {
	// resvg is installed on demand via the tool registry (catalog).
	return nil
}
func (p Provider) New(ctx context.Context) (svgraster.Driver, error) {
	return &Driver{}, nil
}

func init() {
	driver.Register[svgraster.Driver](Provider{})
}
