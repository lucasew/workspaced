package is

import (
	"context"
	"errors"

	envdriver "github.com/lucasew/workspaced/pkg/driver/env"
)

var ErrNotInStore = errors.New("not in store")

type InStore struct{}

func (InStore) Description() string { return "Check if dotfiles are in nix store" }
func (*InStore) Run(ctx context.Context) error {
	if !envdriver.IsInStore(ctx) {
		return ErrNotInStore
	}
	return nil
}
