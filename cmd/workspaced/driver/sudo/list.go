package sudo

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/lucasew/workspaced/internal/sudo"
)

type List struct{}

func (List) Description() string { return "List pending commands" }

func (*List) Run(ctx context.Context) error {
	cmds, err := sudo.List(ctx)
	if err != nil {
		return err
	}

	sort.Slice(cmds, func(i, j int) bool {
		return cmds[i].Timestamp < cmds[j].Timestamp
	})

	if len(cmds) == 0 {
		fmt.Println("No pending commands.")
		return nil
	}

	fmt.Printf("%-15s %-10s %s\n", "SLUG", "TIME", "COMMAND")
	for _, c := range cmds {
		t := time.Unix(c.Timestamp, 0).Format("15:04:05")
		fullCmd := append([]string{c.Command}, c.Args...)
		fmt.Printf("%-15s %-10s %v\n", c.Slug, t, fullCmd)
	}
	return nil
}
