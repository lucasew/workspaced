package drivers

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/lucasew/workspaced/pkg/palette"
)

type Command struct{}

func (Command) Description() string { return "List available palette extraction drivers" }

func (*Command) Run(ctx context.Context) error {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	if _, err := fmt.Fprintln(w, "NAME\tDESCRIPTION"); err != nil {
		return err
	}
	for _, d := range palette.ListDrivers() {
		if _, err := fmt.Fprintf(w, "%s\t%s\n", d.Name(), d.Description()); err != nil {
			return err
		}
	}
	return w.Flush()
}
