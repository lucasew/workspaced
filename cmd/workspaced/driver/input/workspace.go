package input

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"

	"github.com/lewtec/lewkit/x/cmd"
	"github.com/lucasew/workspaced/internal/configcue"
	"github.com/lucasew/workspaced/pkg/driver"
	"github.com/lucasew/workspaced/pkg/driver/dialog"
	"github.com/lucasew/workspaced/pkg/driver/wm"
	"github.com/lucasew/workspaced/pkg/filespine"
)

type Workspace struct {
	Move cmd.Flag `long:"move" help:"Move container to workspace"`
}

func (Workspace) Description() string { return "Workspace switcher" }

func (c *Workspace) Run(ctx context.Context) error {
	result, err := configcue.Evaluate(ctx, configcue.DiscoverOptions{
		HomeLayers: true,
		Mode:       filespine.ModeHome,
	})
	if err != nil {
		return err
	}
	var raw struct {
		Workspaces map[string]int `json:"workspaces"`
	}
	if err := json.Unmarshal(result.JSON, &raw); err != nil {
		return fmt.Errorf("decode evaluated config: %w", err)
	}

	var items []dialog.Item
	var keys []string
	for k := range raw.Workspaces {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		items = append(items, dialog.Item{
			Label: k,
			Value: strconv.Itoa(raw.Workspaces[k]),
		})
	}

	d, err := driver.Get[dialog.Driver](ctx)
	if err != nil {
		return err
	}

	selected, err := d.Choose(ctx, dialog.Options{
		Prompt: "Workspace",
		Items:  items,
	})
	if err != nil {
		return err
	}

	if selected == nil {
		return nil
	}

	return wm.SwitchToWorkspace(ctx, selected.Value, c.Move.Value())
}
