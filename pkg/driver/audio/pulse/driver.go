package pulse

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/lucasew/workspaced/pkg/driver"
	"github.com/lucasew/workspaced/pkg/driver/audio"
	execdriver "github.com/lucasew/workspaced/pkg/driver/exec"
	"github.com/lucasew/workspaced/pkg/logging"
)

var sink = "@DEFAULT_SINK@"

func init() {
	driver.Register[audio.Driver](&Factory{})
}

type Factory struct{}

func (f *Factory) ID() string   { return "audio_pulse" }
func (f *Factory) Name() string { return "PulseAudio (pactl)" }

func (f *Factory) CheckCompatibility(ctx context.Context) error {
	return execdriver.RequireBinary(ctx, "pactl")
}

func (f *Factory) New(ctx context.Context) (audio.Driver, error) {
	return &Driver{}, nil
}

type Driver struct{}

func (d *Driver) SetVolume(ctx context.Context, level float64) error {
	logger := logging.GetLogger(ctx)
	logger.Info("set_volume", "level", level)
	if err := execdriver.MustRun(ctx, "pactl", "set-sink-volume", sink, fmt.Sprintf("%d%%", int(level*100))).Run(); err != nil {
		return fmt.Errorf("set volume: %w", err)
	}
	return nil
}

func parseVolume(output string) (float64, error) {
	volumeStr := strings.TrimSpace(string(output))
	for item := range strings.SplitSeq(volumeStr, " ") {
		if before, ok := strings.CutSuffix(item, "%"); ok {
			volumeStr = before
			volume, err := strconv.Atoi(volumeStr)
			if err != nil {
				return 0, err
			}
			return float64(volume) / 100, nil
		}
	}
	return 0, nil
}

func (d *Driver) GetVolume(ctx context.Context) (float64, error) {
	volumeOut, err := execdriver.MustRun(ctx, "pactl", "get-sink-volume", sink).Output()
	if err != nil {
		return 0, err
	}
	return parseVolume(string(volumeOut))
}

func (d *Driver) GetMute(ctx context.Context) (bool, error) {
	muteOut, err := execdriver.MustRun(ctx, "pactl", "get-sink-mute", sink).Output()
	if err != nil {
		return false, err
	}
	return strings.Contains(string(muteOut), "yes"), nil
}

func (d *Driver) SinkName(ctx context.Context) (string, error) {
	nameOut, err := execdriver.MustRun(ctx, "pactl", "get-default-sink").Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(nameOut)), nil
}

func (d *Driver) ToggleMute(ctx context.Context) error {
	mute, err := d.GetMute(ctx)
	if err != nil {
		return err
	}
	if mute {
		return execdriver.MustRun(ctx, "pactl", "set-sink-mute", sink, "no").Run()
	}
	return execdriver.MustRun(ctx, "pactl", "set-sink-mute", sink, "yes").Run()
}
