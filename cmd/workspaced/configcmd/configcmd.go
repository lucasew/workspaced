// Package configcmd is the shared "config" command tree used by both
// "workspaced home config" and "workspaced codebase config".
// The only behavioral fork is HomeMode (which layers are discovered / which
// Load* helper is used) plus the scope name embedded in example text.
package configcmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/lewtec/lewkit/x/cmd"
	"github.com/lucasew/workspaced/internal/clirun"
	"github.com/lucasew/workspaced/internal/configcue"
	"github.com/lucasew/workspaced/pkg/filespine"
)

// Options selects home vs codebase config discovery.
type Options struct {
	// HomeMode discovers dotfiles/user/home layers instead of the repo layer.
	HomeMode bool
	// Scope is the CLI path segment shown in examples ("home" or "codebase").
	Scope string
}

// Mode selects home vs codebase behavior for Tree[M].
type Mode interface {
	Options() Options
}

// Home is the home/dotfiles config mode.
type Home struct{}

func (Home) Options() Options { return Options{HomeMode: true, Scope: "home"} }

// Codebase is the repo-local config mode.
type Codebase struct{}

func (Codebase) Options() Options { return Options{HomeMode: false, Scope: "codebase"} }

func (o Options) discover() (configcue.DiscoverOptions, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return configcue.DiscoverOptions{}, err
	}
	mode := filespine.ModeCodebase
	if o.HomeMode {
		mode = filespine.ModeHome
	}
	return configcue.DiscoverOptions{Cwd: cwd, HomeLayers: o.HomeMode, Mode: mode}, nil
}

func (o Options) load(ctx context.Context) (*configcue.Config, error) {
	if o.HomeMode {
		return configcue.LoadHome(ctx)
	}
	return configcue.Load(ctx)
}

// Tree is the shared config subcommand group.
type Tree[M Mode] struct {
	Dump   *Dump[M]
	Get    *Get[M]
	Eval   *Eval[M]
	Def    *Def[M]
	Layers *Layers[M]
}

func (Tree[M]) Description() string { return "Manage configuration" }

func (c *Tree[M]) Run(ctx context.Context) error {
	var m M
	return clirun.PrintUsage[Tree[M]]("workspaced " + m.Options().Scope + " config")
}

// Dump prints the full merged configuration as JSON.
type Dump[M Mode] struct{}

func (Dump[M]) Description() string {
	return "Dump the full merged configuration as JSON"
}

func (d *Dump[M]) Run(ctx context.Context) error {
	var m M
	disc, err := m.Options().discover()
	if err != nil {
		return err
	}
	result, err := configcue.Evaluate(ctx, disc)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	var raw any
	if err := json.Unmarshal(result.JSON, &raw); err != nil {
		return fmt.Errorf("decode evaluated config: %w", err)
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(raw)
}

// Get prints one configuration value as JSON.
type Get[M Mode] struct {
	key cmd.StringArg
}

func (Get[M]) Description() string {
	var m M
	scope := m.Options().Scope
	if scope == "" {
		scope = "config"
	}
	return fmt.Sprintf(`Get a configuration value (outputs JSON)

Examples:
  workspaced %s config get workspaces.www
  workspaced %s config get desktop.wallpaper.dir
  workspaced %s config get desktop.wallpaper`, scope, scope, scope)
}

func (g *Get[M]) Run(ctx context.Context) error {
	var m M
	cfg, err := m.Options().load(ctx)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	result, err := lookupConfigValue(cfg, g.key.Value())
	if err != nil {
		return err
	}

	jsonBytes, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("encode JSON: %w", err)
	}
	_, err = fmt.Fprintln(os.Stdout, string(jsonBytes))
	return err
}

func lookupConfigValue(cfg *configcue.Config, key string) (any, error) {
	if key == "" {
		return cfg.Raw(), nil
	}
	return cfg.Lookup(key)
}

// Eval prints the merged config in CUE form.
type Eval[M Mode] struct{}

func (Eval[M]) Description() string {
	return "Evaluate merged config (cue eval-like)"
}

func (e *Eval[M]) Run(ctx context.Context) error {
	return runExport[M](ctx, configcue.ExportCUE)
}

// Def prints merged config definitions/types.
type Def[M Mode] struct{}

func (Def[M]) Description() string {
	return "Show merged config definitions/types (cue def-like)"
}

func (d *Def[M]) Run(ctx context.Context) error {
	return runExport[M](ctx, configcue.ExportDef)
}

func runExport[M Mode](ctx context.Context, export func(context.Context, configcue.DiscoverOptions) ([]byte, error)) error {
	var m M
	disc, err := m.Options().discover()
	if err != nil {
		return err
	}
	out, err := export(ctx, disc)
	if err != nil {
		return err
	}
	_, err = os.Stdout.Write(out)
	return err
}

// Layers lists discovered workspaced.cue layers.
type Layers[M Mode] struct {
	Format cmd.StringArg `short:"f" long:"format" help:"Output format (paths, table)" default:"paths"`
}

func (Layers[M]) Description() string { return "List discovered workspaced.cue layers" }

func (l *Layers[M]) Run(ctx context.Context) error {
	var m M
	disc, err := m.Options().discover()
	if err != nil {
		return err
	}
	result, err := configcue.Evaluate(ctx, disc)
	if err != nil {
		return fmt.Errorf("discover config layers: %w", err)
	}

	format := l.Format.Value()
	if format == "table" {
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		if _, err := fmt.Fprintln(w, "NAME\tPATH"); err != nil {
			return err
		}
		for _, layer := range result.Layers {
			if _, err := fmt.Fprintf(w, "%s\t%s\n", layer.Name, layer.Path); err != nil {
				return err
			}
		}
		return w.Flush()
	}
	if format != "" && format != "paths" {
		return fmt.Errorf("unknown format: %s (supported: paths, table)", format)
	}
	for _, layer := range result.Layers {
		if _, err := fmt.Fprintln(os.Stdout, layer.Path); err != nil {
			return err
		}
	}
	return nil
}
