package is

import (
	"errors"
	"workspaced/pkg/env"

	"github.com/spf13/cobra"
)

var ErrNotPhone = errors.New("not phone")

func init() {
	Registry.Register(func(parent *cobra.Command) {
		parent.AddCommand(&cobra.Command{
			Use:   "phone",
			Short: "Check if environment is a phone",
			RunE: func(c *cobra.Command, args []string) error {
				if !env.IsPhone(c.Context()) {
					return ErrNotPhone
				}
				return nil
			},
		})
	})
}
