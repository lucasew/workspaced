package icons

import (
	"context"
	"os"

	"github.com/lewtec/lewkit/x/cmd"
	iconspkg "github.com/lucasew/workspaced/internal/icons"
)

type Generate struct {
	InputDir       cmd.StringArg   `long:"input-dir" help:"Directory containing .svg/.svg.tmpl masters" default:"~/.dotfiles/assets/icons/master"`
	OutputDir      cmd.StringArg   `long:"output-dir" help:"Output icon theme directory" default:"~/.local/share/icons/workspaced-base16"`
	ThemeName      cmd.StringArg   `long:"theme-name" help:"Theme name written in index.theme" default:"workspaced-base16"`
	Sizes          cmd.StringArg   `long:"sizes" help:"PNG sizes to render, comma-separated" default:"16,24,32,48,64,128,256"`
	Replace        []cmd.StringArg `long:"replace" help:"Color replacement rule old=new (hex, with or without #). Can be repeated"`
	MapScheme      cmd.Flag        `long:"map-scheme" help:"Map all SVG hex colors to nearest color in current base16 scheme" default:"true"`
	NoMapScheme    cmd.Flag        `long:"no-map-scheme" help:"Do not map SVG hex colors to nearest color in current base16 scheme"`
	DefaultContext cmd.StringArg   `long:"default-context" help:"Context to use when icon file is at input root" default:"apps"`
	Clean          cmd.Flag        `long:"clean" help:"Delete output directory before generation"`
	NoRaster       cmd.Flag        `long:"no-raster" help:"Only write scalable SVG icons"`
	UpdateCache    cmd.Flag        `long:"update-cache" help:"Run gtk-update-icon-cache after generation (if available)" default:"true"`
	NoUpdateCache  cmd.Flag        `long:"no-update-cache" help:"Do not run gtk-update-icon-cache after generation"`
	Jobs           cmd.StringArg   `long:"jobs" help:"Number of SVG processing workers (integer or 'auto')" default:"auto"`
}

func (Generate) Description() string {
	return `Generate icon theme variants from SVG templates

Generate a freedesktop icon theme from SVG master files.

Input files can be plain .svg or .svg.tmpl templates.
Template variables include base16 keys (base00..base0F) from the active workspaced config.
Example template usage: fill="#{{ .base0D }}" or fill="%BASE0D%".`
}

func (g *Generate) Run(ctx context.Context) error {
	return iconspkg.RunThemeGenerate(ctx, iconspkg.ThemeGenerateOptions{
		InputDir:       g.InputDir.Value(),
		OutputDir:      g.OutputDir.Value(),
		ThemeName:      g.ThemeName.Value(),
		Jobs:           g.Jobs.Value(),
		Sizes:          g.Sizes.Value(),
		Replace:        cmd.Values(g.Replace),
		MapScheme:      g.MapScheme.Value() && !g.NoMapScheme.Value(),
		HasMapScheme:   true,
		Clean:          g.Clean.Value(),
		NoRaster:       g.NoRaster.Value(),
		UpdateCache:    g.UpdateCache.Value() && !g.NoUpdateCache.Value(),
		HasUpdateCache: true,
		DefaultContext: g.DefaultContext.Value(),
		UseCache:       false,
		Stdout:         os.Stdout,
		Stderr:         os.Stderr,
	})
}
