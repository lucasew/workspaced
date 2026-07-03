package terminal

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
	"workspaced/pkg/driver"
	"workspaced/pkg/driver/dialog"

	"github.com/ktr0731/go-fuzzyfinder"

	"workspaced/pkg/api"
)

func init() {
	driver.Register[dialog.Chooser](&ChooserFactory{})
	driver.Register[dialog.Prompter](&PrompterFactory{})
	driver.Register[dialog.Confirmer](&ConfirmerFactory{})
}

type baseFactory struct{}

func (f *baseFactory) ID() string { return "terminal" }

func (f *baseFactory) CheckCompatibility(ctx context.Context) error {
	// Always compatible, but with weight 0 so it acts as fallback
	return nil
}

type ChooserFactory struct{ baseFactory }

func (f *ChooserFactory) Name() string                                    { return "Terminal (Fuzzy)" }
func (f *ChooserFactory) New(ctx context.Context) (dialog.Chooser, error) { return &Driver{}, nil }

type PrompterFactory struct{ baseFactory }

func (f *PrompterFactory) Name() string                                     { return "Terminal (Stdin)" }
func (f *PrompterFactory) New(ctx context.Context) (dialog.Prompter, error) { return &Driver{}, nil }

type ConfirmerFactory struct{ baseFactory }

func (f *ConfirmerFactory) Name() string                                      { return "Terminal (y/n)" }
func (f *ConfirmerFactory) New(ctx context.Context) (dialog.Confirmer, error) { return &Driver{}, nil }

// Driver implements Chooser, Prompter and Confirmer
type Driver struct{}

func (d *Driver) Choose(ctx context.Context, opts dialog.ChooseOptions) (*dialog.Item, error) {
	idx, err := fuzzyfinder.Find(
		opts.Items,
		func(i int) string {
			return opts.Items[i].Label
		},
	)
	if err != nil {
		if err == fuzzyfinder.ErrAbort {
			return nil, nil
		}
		return nil, err
	}
	return &opts.Items[idx], nil
}

func (d *Driver) Prompt(ctx context.Context, prompt string) (string, error) {
	fmt.Printf("%s: ", prompt)
	scanner := bufio.NewScanner(os.Stdin)
	if scanner.Scan() {
		return strings.TrimSpace(scanner.Text()), nil
	}
	return "", scanner.Err()
}

func (d *Driver) Confirm(ctx context.Context, message string) (bool, error) {
	fmt.Printf("%s [y/N]: ", message)
	scanner := bufio.NewScanner(os.Stdin)
	if scanner.Scan() {
		text := strings.ToLower(strings.TrimSpace(scanner.Text()))
		return text == "y" || text == "yes", nil
	}
	return false, scanner.Err()
}

// Legacy compatibility for Driver interface
func (d *Driver) RunApp(ctx context.Context) error {
	return api.ErrNotImplemented
}
func (d *Driver) SwitchWindow(ctx context.Context) error {
	return api.ErrNotImplemented
}
