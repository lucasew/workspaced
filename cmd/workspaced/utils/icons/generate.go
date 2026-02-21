package icons

import (
	"os"
	iconspkg "workspaced/pkg/icons"

	"github.com/spf13/cobra"
)

func init() {
	Registry.FromGetter(GetGenerateCommand)
}

func GetGenerateCommand() *cobra.Command {
	var (
		inputDir       string
		outputDir      string
		themeName      string
		sizesRaw       string
		replacements   []string
		mapScheme      bool
		clean          bool
		noRaster       bool
		updateCache    bool
		defaultContext string
		jobsRaw        string
	)

	cmd := &cobra.Command{
		Use:   "generate",
		Short: "Generate icon theme variants from SVG templates",
		Long: `Generate a freedesktop icon theme from SVG master files.

Input files can be plain .svg or .svg.tmpl templates.
Template variables include base16 keys (base00..base0F) from settings.toml.
Example template usage: fill="#{{ .base0D }}" or fill="%BASE0D%".`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return iconspkg.RunThemeGenerate(cmd.Context(), iconspkg.ThemeGenerateOptions{
				InputDir:       inputDir,
				OutputDir:      outputDir,
				ThemeName:      themeName,
				Jobs:           jobsRaw,
				Sizes:          sizesRaw,
				Replace:        replacements,
				MapScheme:      mapScheme,
				HasMapScheme:   true,
				Clean:          clean,
				NoRaster:       noRaster,
				UpdateCache:    updateCache,
				HasUpdateCache: true,
				DefaultContext: defaultContext,
				UseCache:       false,
				Stdout:         os.Stdout,
				Stderr:         os.Stderr,
			})
		},
	}

	cmd.Flags().StringVar(&inputDir, "input-dir", "~/.dotfiles/assets/icons/master", "Directory containing .svg/.svg.tmpl masters")
	cmd.Flags().StringVar(&outputDir, "output-dir", "~/.local/share/icons/workspaced-base16", "Output icon theme directory")
	cmd.Flags().StringVar(&themeName, "theme-name", "workspaced-base16", "Theme name written in index.theme")
	cmd.Flags().StringVar(&sizesRaw, "sizes", "16,24,32,48,64,128,256", "PNG sizes to render, comma-separated")
	cmd.Flags().StringArrayVar(&replacements, "replace", nil, "Color replacement rule old=new (hex, with or without #). Can be repeated")
	cmd.Flags().BoolVar(&mapScheme, "map-scheme", true, "Map all SVG hex colors to nearest color in current base16 scheme")
	cmd.Flags().StringVar(&defaultContext, "default-context", "apps", "Context to use when icon file is at input root")
	cmd.Flags().BoolVar(&clean, "clean", false, "Delete output directory before generation")
	cmd.Flags().BoolVar(&noRaster, "no-raster", false, "Only write scalable SVG icons")
	cmd.Flags().BoolVar(&updateCache, "update-cache", true, "Run gtk-update-icon-cache after generation (if available)")
	cmd.Flags().StringVar(&jobsRaw, "jobs", "auto", "Number of SVG processing workers (integer or 'auto')")
	return cmd
}
