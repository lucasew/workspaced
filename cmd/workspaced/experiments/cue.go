package experiments

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/lucasew/workspaced/internal/clirun"
	"github.com/lucasew/workspaced/internal/configcue"
)

type Cue struct {
	Layers *CueLayers `cmd:"layers"`
	Export *CueExport `cmd:"export"`
}

func (Cue) Description() string {
	return "Inspect experimental layered CUE configuration"
}

func (c *Cue) Run(ctx context.Context) error {
	return clirun.PrintUsage[Cue]("workspaced experiments cue")
}

type CueLayers struct{}

func (CueLayers) Description() string { return "List discovered workspaced.cue layers" }

func (*CueLayers) Run(ctx context.Context) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	res, err := configcue.DiscoverLayers(ctx, configcue.DiscoverOptions{Cwd: cwd})
	if err != nil {
		return err
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(res)
}

type CueExport struct{}

func (CueExport) Description() string { return "Export the unified experimental CUE config as JSON" }

func (*CueExport) Run(ctx context.Context) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	data, err := configcue.ExportJSON(ctx, configcue.DiscoverOptions{Cwd: cwd})
	if err != nil {
		return err
	}
	var pretty any
	if err := json.Unmarshal(data, &pretty); err != nil {
		return fmt.Errorf("decode generated json: %w", err)
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(pretty)
}
