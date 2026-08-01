package i3ipc

import (
	"context"
	"encoding/json"
	"fmt"
	dapi "github.com/lucasew/workspaced/pkg/api"
	"github.com/lucasew/workspaced/pkg/driver"
	execdriver "github.com/lucasew/workspaced/pkg/driver/exec"
	api "github.com/lucasew/workspaced/pkg/driver/wm"
)

func init() {
	driver.Register[api.Driver](&SwayFactory{})
	driver.Register[api.Driver](&I3Factory{})
}

type SwayFactory struct{}

func (f *SwayFactory) ID() string   { return "screen_wayland_sway" }
func (f *SwayFactory) Name() string { return "Sway" }
func (f *SwayFactory) CheckCompatibility(ctx context.Context) error {
	return execdriver.RequireEnvBinary(ctx, "WAYLAND_DISPLAY", "swaymsg")
}

func (f *SwayFactory) New(ctx context.Context) (api.Driver, error) {
	return &Driver{Binary: "swaymsg"}, nil
}

type I3Factory struct{}

func (f *I3Factory) ID() string   { return "screen_x11_i3" }
func (f *I3Factory) Name() string { return "i3" }
func (f *I3Factory) CheckCompatibility(ctx context.Context) error {
	return execdriver.RequireEnvBinary(ctx, "DISPLAY", "i3-msg")
}

func (f *I3Factory) New(ctx context.Context) (api.Driver, error) {
	return &Driver{Binary: "i3-msg"}, nil
}

type Driver struct {
	Binary string
}

func (d *Driver) MoveWorkspaceToOutput(ctx context.Context, workspace string, output string) error {
	return execdriver.MustRun(ctx, d.Binary, fmt.Sprintf("[workspace=%q] move workspace to output %q", workspace, output)).Run()
}

func (d *Driver) SwitchToWorkspace(ctx context.Context, ws string, move bool) error {
	if move {
		return execdriver.MustRun(ctx, d.Binary, "move", "container", "to", "workspace", ws).Run()
	}
	return execdriver.MustRun(ctx, d.Binary, "workspace", ws).Run()
}

func (d *Driver) ToggleScratchpad(ctx context.Context) error {
	return execdriver.MustRun(ctx, d.Binary, "scratchpad", "show").Run()
}

func (d *Driver) GetOutputs(ctx context.Context) ([]api.Output, error) {
	return getIPCJSON[[]api.Output](ctx, d.Binary, "get_outputs")
}

func (d *Driver) GetWorkspaces(ctx context.Context) ([]api.Workspace, error) {
	return getIPCJSON[[]api.Workspace](ctx, d.Binary, "get_workspaces")
}

// getIPCJSON runs `binary -t <msg>` and unmarshals the JSON reply into T.
func getIPCJSON[T any](ctx context.Context, binary, msg string) (T, error) {
	var zero T
	out, err := execdriver.MustRun(ctx, binary, "-t", msg).Output()
	if err != nil {
		return zero, fmt.Errorf("%w: %w", dapi.ErrIPC, err)
	}
	var v T
	if err := json.Unmarshal(out, &v); err != nil {
		return zero, fmt.Errorf("%w: %w", dapi.ErrIPC, err)
	}
	return v, nil
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

	workspaces, err := d.GetWorkspaces(ctx)
	if err != nil {
		return "", nil, err
	}

	var focusedOutputName string
	for _, w := range workspaces {
		if w.Focused {
			focusedOutputName = w.Output
			break
		}
	}

	if focusedOutputName != "" {
		for _, o := range outputs {
			if o.Name == focusedOutputName {
				return o.Name, &o.Rect, nil
			}
		}
	}

	return "", nil, dapi.ErrNoFocusedOutput
}

func (d *Driver) GetFocusedWindowRect(ctx context.Context) (*api.Rect, error) {
	out, err := execdriver.MustRun(ctx, d.Binary, "-t", "get_tree").Output()
	if err != nil {
		return nil, fmt.Errorf("%w: %w", dapi.ErrIPC, err)
	}

	var root api.Node
	if err := json.Unmarshal(out, &root); err != nil {
		return nil, fmt.Errorf("%w: %w", dapi.ErrIPC, err)
	}

	found := findFocusedNode(&root)
	if found != nil {
		return &found.Rect, nil
	}

	return nil, dapi.ErrNoFocusedWindow
}

func findFocusedNode(node *api.Node) *api.Node {
	if node.Focused {
		return node
	}
	for _, n := range node.Nodes {
		if found := findFocusedNode(n); found != nil {
			return found
		}
	}
	for _, n := range node.FloatingNodes {
		if found := findFocusedNode(n); found != nil {
			return found
		}
	}
	return nil
}
