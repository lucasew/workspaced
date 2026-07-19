package drivers

import (
	"fmt"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/lucasew/workspaced/pkg/palette"
)

func GetCommand() *cobra.Command {
	return &cobra.Command{
		Use:     "drivers",
		Short:   "List available palette extraction drivers",
		Aliases: []string{"list-drivers"},
		RunE: func(cmd *cobra.Command, _ []string) error {
			w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			if _, err := fmt.Fprintln(w, "NAME\tDESCRIPTION"); err != nil {
				return err
			}
			for _, d := range palette.ListDrivers() {
				if _, err := fmt.Fprintf(w, "%s\t%s\n", d.Name(), d.Description()); err != nil {
					return err
				}
			}
			return w.Flush()
		},
	}
}
