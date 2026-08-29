package dialog

import (
	"context"

	"github.com/lucasew/workspaced/pkg/driver"
)

// Choose allows selecting an item from a list. It tries graphical choosers first.
func Choose(ctx context.Context, opts ChooseOptions) (*Item, error) {
	return driver.WithResult(ctx, func(d Chooser) (*Item, error) { return d.Choose(ctx, opts) })
}

// Prompt asks for a simple text input.
func Prompt(ctx context.Context, prompt string) (string, error) {
	return driver.WithResult(ctx, func(d Prompter) (string, error) { return d.Prompt(ctx, prompt) })
}

// Confirm asks a yes/no question.
func Confirm(ctx context.Context, message string) (bool, error) {
	return driver.WithResult(ctx, func(d Confirmer) (bool, error) { return d.Confirm(ctx, message) })
}
