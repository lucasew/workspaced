package brightnessctl

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"workspaced/pkg/driver"
	"workspaced/pkg/driver/brightness"
	execdriver "workspaced/pkg/driver/exec"
)

func init() {
	driver.Register[brightness.Driver](&Factory{})
}

type Factory struct{}

func (f *Factory) ID() string   { return "brightness_ctl" }
func (f *Factory) Name() string { return "brightnessctl" }

func (f *Factory) CheckCompatibility(ctx context.Context) error {
	return execdriver.RequireBinary(ctx, "brightnessctl")
}

func (f *Factory) New(ctx context.Context) (brightness.Driver, error) {
	return &Driver{}, nil
}

type Driver struct{}

// Status implements [brightness.Driver].
func (d *Driver) Status(ctx context.Context) (*brightness.Device, error) {
	out, err := execdriver.MustRun(ctx, "brightnessctl", "-m").Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get brightness status: %w", err)
	}

	lines := strings.SplitSeq(strings.TrimSpace(string(out)), "\n")
	for line := range lines {
		parts := strings.Split(line, ",")
		if len(parts) < 5 {
			continue
		}
		devname := parts[0]
		level := parts[3]

		levelVal := strings.TrimSuffix(level, "%")
		l, err := strconv.Atoi(levelVal)
		if err != nil {
			continue
		}
		return &brightness.Device{
			Name:       devname,
			Brightness: float64(l) / 100,
		}, nil
	}
	return nil, brightness.ErrDeviceNotFound
}

func (d *Driver) SetBrightness(ctx context.Context, level float64) error {

	if err := execdriver.MustRun(ctx, "brightnessctl", "s", fmt.Sprintf("%d%%", int(level*100))).Run(); err != nil {
		return fmt.Errorf("failed to set brightness: %w", err)
	}

	return nil
}
