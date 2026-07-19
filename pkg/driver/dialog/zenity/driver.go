package zenity

import (
	"context"
	"fmt"
	"strings"
	"github.com/lucasew/workspaced/internal/executil"
	"github.com/lucasew/workspaced/pkg/driver"
	"github.com/lucasew/workspaced/pkg/driver/dialog"
	execdriver "github.com/lucasew/workspaced/pkg/driver/exec"
)

func init() {
	driver.Register[dialog.Prompter](&PrompterFactory{})
	driver.Register[dialog.Confirmer](&ConfirmerFactory{})
}

type baseFactory struct{}

func (f *baseFactory) ID() string { return "zenity" }

func (f *baseFactory) CheckCompatibility(ctx context.Context) error {
	if executil.GetEnv(ctx, "DISPLAY") == "" && executil.GetEnv(ctx, "WAYLAND_DISPLAY") == "" {
		return fmt.Errorf("%w: neither DISPLAY nor WAYLAND_DISPLAY set", driver.ErrIncompatible)
	}
	if !execdriver.IsBinaryAvailable(ctx, "zenity") {
		return fmt.Errorf("%w: zenity not found", driver.ErrIncompatible)
	}
	return nil
}

type PrompterFactory struct{ baseFactory }

func (f *PrompterFactory) Name() string                                     { return "Zenity (Prompt)" }
func (f *PrompterFactory) New(ctx context.Context) (dialog.Prompter, error) { return &Driver{}, nil }

type ConfirmerFactory struct{ baseFactory }

func (f *ConfirmerFactory) Name() string                                      { return "Zenity (Confirm)" }
func (f *ConfirmerFactory) New(ctx context.Context) (dialog.Confirmer, error) { return &Driver{}, nil }

type Driver struct{}

func (d *Driver) Prompt(ctx context.Context, prompt string) (string, error) {
	out, err := execdriver.MustRun(ctx, "zenity", "--entry", "--text", prompt).Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func (d *Driver) Confirm(ctx context.Context, message string) (bool, error) {
	err := execdriver.MustRun(ctx, "zenity", "--question", "--text", message).Run()
	if err != nil {
		// Zenity returns non-zero if No is selected
		return false, nil
	}
	return true, nil
}
