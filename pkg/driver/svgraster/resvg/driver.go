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
	"workspaced/pkg/tool"
)

const defaultResvgSpec = "github:linebender/resvg@latest"

type Driver struct{}

func (d *Driver) RasterizeSVG(ctx context.Context, svg string, width int, height int) (image.Image, error) {
	tmpDir, err := os.MkdirTemp("", "workspaced-svgraster-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmpDir)

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
	defer f.Close()

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
func (p Provider) DefaultWeight() int { return 100 }
func (p Provider) CheckCompatibility(ctx context.Context) error {
	// resvg is installed on demand via the tool subsystem.
	return nil
}
func (p Provider) New(ctx context.Context) (svgraster.Driver, error) {
	return &Driver{}, nil
}

func init() {
	driver.Register[svgraster.Driver](Provider{})
}
