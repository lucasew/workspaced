package open

import (
	"context"

	"github.com/lewtec/lewkit/x/cmd"
	"github.com/lucasew/workspaced/internal/clirun"
	"github.com/lucasew/workspaced/internal/configcue"
	"github.com/lucasew/workspaced/pkg/driver/opener"
	"github.com/lucasew/workspaced/pkg/driver/terminal"
)

type Command struct {
	File     *File
	Webapp   *Webapp
	Terminal *Terminal
	Exec     *Exec
	Lazy     *Lazy
	Mise     *Mise
}

func (Command) Description() string {
	return "Open a file, URL or webapp"
}

func (c *Command) Run(ctx context.Context) error {
	return clirun.PrintUsage[Command]("workspaced open")
}

type File struct {
	Target cmd.StringArg
}

func (File) Description() string { return "Open a file or URL" }

func (f *File) Run(ctx context.Context) error {
	return opener.Open(ctx, f.Target.Value())
}

type Webapp struct {
	URL       cmd.StringArg   `short:"u" long:"url" help:"URL to open"`
	Profile   cmd.StringArg   `short:"p" long:"profile" help:"Browser profile name"`
	ExtraFlag []cmd.StringArg `short:"e" long:"extra-flag" help:"Extra browser flags"`
	Chromium  cmd.StringArg   `long:"chromium" help:"Chromium-like binary to use (overrides browser.webapp)"`
	name      *cmd.StringArg
}

func (Webapp) Description() string { return "Launch a configured webapp" }

func (w *Webapp) Run(ctx context.Context) error {
	cfg, err := configcue.LoadHome(ctx)
	if err != nil {
		return err
	}

	var wa opener.WebappConfig

	if w.name != nil {
		name := w.name.Value()
		var modCfg struct {
			Apps map[string]opener.WebappConfig `json:"apps"`
		}
		if err := cfg.ModuleConfig("webapp", &modCfg); err == nil {
			if app, ok := modCfg.Apps[name]; ok {
				wa = app
			}
		}
	}

	if url := w.URL.Value(); url != "" {
		wa.URL = url
	}
	if profile := w.Profile.Value(); profile != "" {
		wa.Profile = profile
	}
	if extra := cmd.Values(w.ExtraFlag); len(extra) > 0 {
		wa.ExtraFlags = append(wa.ExtraFlags, extra...)
	}
	if chromium := w.Chromium.Value(); chromium != "" {
		wa.Chromium = chromium
	}

	return opener.OpenWebapp(ctx, wa)
}

type Terminal struct {
	cmd []cmd.StringArg
}

func (Terminal) Description() string { return "Launch the preferred terminal" }

func (t *Terminal) Run(ctx context.Context) error {
	args := cmd.Values(t.cmd)
	opts := terminal.Options{
		Title: "Terminal",
	}
	if len(args) > 0 {
		opts.Command = args[0]
		opts.Args = args[1:]
	}
	return terminal.Open(ctx, opts)
}
