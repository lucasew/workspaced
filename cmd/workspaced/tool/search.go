package tool

import (
	"fmt"
	"strings"

	"workspaced/pkg/tool/backend/catalog"

	"github.com/spf13/cobra"
)

func init() {
	Registry.Register(func(c *cobra.Command) {
		c.AddCommand(&cobra.Command{
			Use:   "search [query]",
			Short: "Search (or list) curated short names in the tool catalog",
			Long: `Search the registry catalog of curated short names (the bare names such as "uv", "ripgrep", "nodejs").

If no query is given, lists every known short name (alphabetical).
If a query is given, prints only names containing the query (case-insensitive match).
These names can be used directly as tool-specs (they resolve via the "registry" backend).

The matching names are the command's final output ("verdict") and are written to stdout.
This means e.g. 'workspaced tool search u 2>/dev/null' will still produce the list on stdout.`,
			Args: cobra.MaximumNArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				query := ""
				if len(args) == 1 {
					query = strings.ToLower(strings.TrimSpace(args[0]))
				}
				for _, name := range catalog.ListTools() {
					if query == "" || strings.Contains(strings.ToLower(name), query) {
						// Final verdict (the list of matches) goes explicitly to the command's stdout.
						fmt.Fprintln(cmd.OutOrStdout(), name)
					}
				}
				return nil
			},
		})
	})
}
