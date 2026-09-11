package tool

import (
	"context"
	"fmt"

	"github.com/lucasew/workspaced/internal/tool"
)

type List struct{}

func (List) Description() string { return "List installed tools" }

func (*List) Run(ctx context.Context) error {
	manager, err := tool.NewManager()
	if err != nil {
		return err
	}
	tools, err := manager.ListInstalled()
	if err != nil {
		return err
	}
	for _, t := range tools {
		fmt.Printf("%s %s\n", t.Name, t.Version)
	}
	return nil
}
