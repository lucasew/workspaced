package rofi

import (
	"context"
	"fmt"
	"strings"
	"workspaced/internal/executil"
	"workspaced/pkg/driver"
	"workspaced/pkg/driver/dialog"
	execdriver "workspaced/pkg/driver/exec"
)

func init() {
	driver.Register[dialog.Chooser](&ChooserFactory{})
	driver.Register[dialog.Driver](&FullDriverFactory{})
}

type ChooserFactory struct{}

func (f *ChooserFactory) ID() string                                      { return "rofi" }
func (f *ChooserFactory) Name() string                                    { return "Rofi" }
func (f *ChooserFactory) CheckCompatibility(ctx context.Context) error    { return checkRofi(ctx) }
func (f *ChooserFactory) New(ctx context.Context) (dialog.Chooser, error) { return &Driver{}, nil }

type FullDriverFactory struct{}

func (f *FullDriverFactory) ID() string                                     { return "rofi" }
func (f *FullDriverFactory) Name() string                                   { return "Rofi" }
func (f *FullDriverFactory) CheckCompatibility(ctx context.Context) error   { return checkRofi(ctx) }
func (f *FullDriverFactory) New(ctx context.Context) (dialog.Driver, error) { return &Driver{}, nil }

func checkRofi(ctx context.Context) error {
	if executil.GetEnv(ctx, "DISPLAY") == "" && executil.GetEnv(ctx, "WAYLAND_DISPLAY") == "" {
		return fmt.Errorf("%w: neither DISPLAY nor WAYLAND_DISPLAY set", driver.ErrIncompatible)
	}
	if !execdriver.IsBinaryAvailable(ctx, "rofi") {
		return fmt.Errorf("%w: rofi not found", driver.ErrIncompatible)
	}
	return nil
}

type Driver struct{}

func (d *Driver) Choose(ctx context.Context, opts dialog.ChooseOptions) (*dialog.Item, error) {
	var input strings.Builder
	for _, item := range opts.Items {
		label := item.Label
		if label == "" {
			label = item.Value
		}
		input.WriteString(label)
		if item.Icon != "" {
			input.WriteString("\x00icon\x1f")
			input.WriteString(item.Icon)
		}
		input.WriteString("\n")
	}

	args := []string{"-dmenu", "-p", opts.Prompt}
	args = append(args, "-show-icons")

	cmd := execdriver.MustRun(ctx, "rofi", args...)
	cmd.Stdin = strings.NewReader(input.String())

	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	selected := strings.TrimSpace(string(out))
	if selected == "" {
		return nil, nil
	}

	for _, item := range opts.Items {
		label := item.Label
		if label == "" {
			label = item.Value
		}
		if selected == label {
			return &item, nil
		}
	}

	return &dialog.Item{Label: selected, Value: selected}, nil
}

func (d *Driver) RunApp(ctx context.Context) error {
	return execdriver.MustRun(ctx, "rofi", "-show", "combi", "-combi-modi", "drun", "-show-icons").Run()
}

func (d *Driver) SwitchWindow(ctx context.Context) error {
	return execdriver.MustRun(ctx, "rofi", "-show", "combi", "-combi-modi", "window", "-show-icons").Run()
}
