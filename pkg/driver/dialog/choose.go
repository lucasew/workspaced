package dialog

import (
	"context"
	"strings"

	execdriver "github.com/lucasew/workspaced/pkg/driver/exec"
)

// ItemLabel returns Label, or Value when Label is empty.
func ItemLabel(item Item) string {
	if item.Label != "" {
		return item.Label
	}
	return item.Value
}

// FormatChoiceLines writes one dmenu line per item. withIcons appends rofi
// icon metadata when Item.Icon is set.
func FormatChoiceLines(items []Item, withIcons bool) string {
	var b strings.Builder
	for _, item := range items {
		b.WriteString(ItemLabel(item))
		if withIcons && item.Icon != "" {
			b.WriteString("\x00icon\x1f")
			b.WriteString(item.Icon)
		}
		b.WriteString("\n")
	}
	return b.String()
}

// MatchSelected maps dmenu stdout back to an item. Empty selected is cancel.
func MatchSelected(items []Item, selected string) *Item {
	selected = strings.TrimSpace(selected)
	if selected == "" {
		return nil
	}
	for i := range items {
		if ItemLabel(items[i]) == selected {
			return &items[i]
		}
	}
	return &Item{Label: selected, Value: selected}
}

// ChooseViaCmd feeds FormatChoiceLines to name and matches the selected line.
func ChooseViaCmd(ctx context.Context, opts ChooseOptions, name string, withIcons bool, args ...string) (*Item, error) {
	cmd := execdriver.MustRun(ctx, name, args...)
	cmd.Stdin = strings.NewReader(FormatChoiceLines(opts.Items, withIcons))
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	return MatchSelected(opts.Items, string(out)), nil
}
