package generate

import (
	"encoding/json"
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/lucasew/workspaced/pkg/palette"
	"github.com/lucasew/workspaced/pkg/palette/api"
)

func GetCommand() *cobra.Command {
	var (
		driverName string
		polarity   string
		colorCount int
	)

	cmd := &cobra.Command{
		Use:   "generate <image>",
		Short: "Generate color palette from an image",
		Long:  generateLongHelp(),
		Args:  cobra.ExactArgs(1),
		RunE: func(c *cobra.Command, args []string) error {
			ctx := c.Context()
			imagePath := args[0]

			if _, err := palette.GetDriver(ctx, driverName); err != nil {
				return err
			}

			pol, err := parsePolarityFlag(polarity)
			if err != nil {
				return err
			}

			opts := api.Options{
				Polarity:   pol,
				ColorCount: colorCount,
				MaxSamples: 10000,
			}

			pal, err := palette.ExtractFromFile(ctx, imagePath, driverName, opts)
			if err != nil {
				return fmt.Errorf("extract palette: %w", err)
			}

			encoder := json.NewEncoder(c.OutOrStdout())
			encoder.SetIndent("", "  ")
			return encoder.Encode(pal)
		},
	}

	cmd.Flags().StringVar(&driverName, "driver", "genetic", "Extraction algorithm (see: palette drivers)")
	cmd.Flags().StringVar(&polarity, "polarity", "any", "Theme preference: dark, light, or any")
	cmd.Flags().IntVar(&colorCount, "colors", 16, "Number of colors (16 for base16, 24 for base24)")
	_ = cmd.RegisterFlagCompletionFunc("driver", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return palette.DriverNames(), cobra.ShellCompDirectiveNoFileComp
	})
	_ = cmd.RegisterFlagCompletionFunc("polarity", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return []string{"any", "dark", "light"}, cobra.ShellCompDirectiveNoFileComp
	})
	return cmd
}

func generateLongHelp() string {
	var b strings.Builder
	b.WriteString(`Generate a base16 or base24 color palette from an image.

Drivers (see also: workspaced utils palette drivers):
`)
	w := tabwriter.NewWriter(&b, 0, 0, 2, ' ', 0)
	for _, d := range palette.ListDrivers() {
		fmt.Fprintf(w, "  %s\t%s\n", d.Name(), d.Description())
	}
	_ = w.Flush()
	b.WriteString(`
Examples:
  # Generate dark theme from wallpaper (default genetic driver)
  workspaced utils palette generate ~/wallpaper.jpg --polarity dark

  # Material You scheme as base24
  workspaced utils palette generate image.png --driver materialyou --colors 24 --polarity light`)
	return b.String()
}

// parsePolarityFlag converts string flag to Polarity enum
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
