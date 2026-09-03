package hyprland

import (
	"context"
	"fmt"

	dapi "github.com/lucasew/workspaced/pkg/api"
	"github.com/lucasew/workspaced/pkg/driver"
	execdriver "github.com/lucasew/workspaced/pkg/driver/exec"
	api "github.com/lucasew/workspaced/pkg/driver/wm"
)

func init() {
	driver.Register[api.Driver](&Factory{})
}

type Factory struct{}

func (f *Factory) ID() string   { return "wm_hyprland" }
func (f *Factory) Name() string { return "Hyprland" }

func (f *Factory) CheckCompatibility(ctx context.Context) error {
	if err := driver.RequireEnv(ctx, "HYPRLAND_INSTANCE_SIGNATURE"); err != nil {
		return err
	}
	return execdriver.RequireBinary(ctx, "hyprctl")
}

func (f *Factory) New(ctx context.Context) (api.Driver, error) {
	return &Driver{}, nil
}

type Driver struct{}

func (d *Driver) MoveWorkspaceToOutput(ctx context.Context, workspace string, output string) error {
	return execdriver.MustRun(ctx, "hyprctl", "dispatch", "moveworkspacetomonitor", workspace, output).Run()
}

func (d *Driver) SwitchToWorkspace(ctx context.Context, ws string, move bool) error {
	cmd := "workspace"
	if move {
		cmd = "movetoworkspace"
	}
	return execdriver.MustRun(ctx, "hyprctl", "dispatch", cmd, ws).Run()
}

func (d *Driver) ToggleScratchpad(ctx context.Context) error {
	return execdriver.MustRun(ctx, "hyprctl", "dispatch", "togglespecialworkspace").Run()
}

func (d *Driver) GetOutputs(ctx context.Context) ([]api.Output, error) {
	monitors, err := api.JSONViaCmd[[]struct {
		Name            string `json:"name"`
		Focused         bool   `json:"focused"`
		X               int    `json:"x"`
		Y               int    `json:"y"`
		Width           int    `json:"width"`
		Height          int    `json:"height"`
		ActiveWorkspace struct {
			Name string `json:"name"`
		} `json:"activeWorkspace"`
	}](ctx, "hyprctl", "monitors", "-j")
	if err != nil {
		return nil, err
	}
	var outputs []api.Output
	for _, m := range monitors {
		outputs = append(outputs, api.Output{
			Name:             m.Name,
			Focused:          m.Focused,
			CurrentWorkspace: m.ActiveWorkspace.Name,
			Rect:             api.Rect{X: m.X, Y: m.Y, Width: m.Width, Height: m.Height},
		})
	}
	return outputs, nil
}

func (d *Driver) GetWorkspaces(ctx context.Context) ([]api.Workspace, error) {
	workspaces, err := api.JSONViaCmd[[]struct {
		Name    string `json:"name"`
		Monitor string `json:"monitor"`
	}](ctx, "hyprctl", "workspaces", "-j")
	if err != nil {
		return nil, err
	}

	activeWS, err := api.JSONViaCmd[struct {
		Name string `json:"name"`
	}](ctx, "hyprctl", "activeworkspace", "-j")
	if err != nil {
		return nil, err
	}

	var result []api.Workspace
	for _, w := range workspaces {
		result = append(result, api.Workspace{
			Name:    w.Name,
			Output:  w.Monitor,
			Focused: w.Name == activeWS.Name,
		})
	}
	return result, nil
}

func (d *Driver) GetFocusedOutput(ctx context.Context) (string, *api.Rect, error) {
	outputs, err := d.GetOutputs(ctx)
	if err != nil {
		return "", nil, err
	}
	for _, o := range outputs {
		if o.Focused {
			return o.Name, &o.Rect, nil
		}
	}
	return "", nil, dapi.ErrNoFocusedOutput
}

func (d *Driver) GetFocusedWindowRect(ctx context.Context) (*api.Rect, error) {
	win, err := api.JSONViaCmd[struct {
		At   []int `json:"at"`
		Size []int `json:"size"`
	}](ctx, "hyprctl", "activewindow", "-j")
	if err != nil {
		return nil, err
	}
	if len(win.At) != 2 || len(win.Size) != 2 {
		return nil, fmt.Errorf("%w: invalid hyprland active window geometry", dapi.ErrIPC)
	}
	return &api.Rect{X: win.At[0], Y: win.At[1], Width: win.Size[0], Height: win.Size[1]}, nil
}
