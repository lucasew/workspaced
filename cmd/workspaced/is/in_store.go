package is

import (
	"errors"
	"workspaced/pkg/env"

	"github.com/spf13/cobra"
)

var ErrNotInStore = errors.New("not in store")

func init() {
	Registry.Register(func(parent *cobra.Command) {
		parent.AddCommand(&cobra.Command{
			Use:   "in-store",
			Short: "Check if dotfiles are in nix store",
			RunE: func(c *cobra.Command, args []string) error {
				if !env.IsInStore(c.Context()) {
					return ErrNotInStore
				}
				return nil
			},
		})
	})
}
