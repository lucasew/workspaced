package generate

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/lewtec/lewkit/x/cmd"
	"github.com/lucasew/workspaced/pkg/palette"
	"github.com/lucasew/workspaced/pkg/palette/api"
)

type Command struct {
	Driver   cmd.StringArg   `long:"driver" help:"Extraction algorithm (see: palette drivers)" default:"genetic"`
	Polarity cmd.StringArg   `long:"polarity" help:"Theme preference: dark, light, or any" default:"any"`
	Colors   cmd.IntArg[int] `long:"colors" help:"Number of colors (16 for base16, 24 for base24)" default:"16"`
	image    cmd.StringArg
}

func (Command) Description() string { return generateLongHelp() }

func (c *Command) Run(ctx context.Context) error {
	imagePath := c.image.Value()

	if _, err := palette.GetDriver(ctx, c.Driver.Value()); err != nil {
		return err
	}

	pol, err := parsePolarityFlag(c.Polarity.Value())
	if err != nil {
		return err
	}

	opts := api.Options{
		Polarity:   pol,
		ColorCount: c.Colors.Value(),
		MaxSamples: 10000,
	}

	pal, err := palette.ExtractFromFile(ctx, imagePath, c.Driver.Value(), opts)
	if err != nil {
		return fmt.Errorf("extract palette: %w", err)
	}

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(pal)
}

func generateLongHelp() string {
	var b strings.Builder
	b.WriteString(`Generate color palette from an image

Generate a base16 or base24 color palette from an image.

Drivers (see also: workspaced utils palette drivers):
`)
	w := tabwriter.NewWriter(&b, 0, 0, 2, ' ', 0)
	for _, d := range palette.ListDrivers() {
		fmt.Fprintf(w, "  %s\t%s\n", d.Name(), d.Description())
	}
	if flushErr := w.Flush(); flushErr != nil {
		// best-effort for help text formatting
	}
	b.WriteString(`
Examples:
  # Generate dark theme from wallpaper (default genetic driver)
  workspaced utils palette generate ~/wallpaper.jpg --polarity dark

  # Material You scheme as base24
  workspaced utils palette generate image.png --driver materialyou --colors 24 --polarity light`)
	return b.String()
}

func parsePolarityFlag(s string) (api.Polarity, error) {
	switch strings.ToLower(s) {
	case "any":
		return api.PolarityAny, nil
	case "dark":
		return api.PolarityDark, nil
	case "light":
		return api.PolarityLight, nil
	default:
		return api.PolarityAny, fmt.Errorf("invalid polarity: %s (must be 'dark', 'light', or 'any')", s)
	}
}
