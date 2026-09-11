package is

import (
	"context"
	"fmt"

	"github.com/lewtec/lewkit/x/cmd"
	"github.com/lucasew/workspaced/internal/clirun"
	envdriver "github.com/lucasew/workspaced/pkg/driver/env"
	execdriver "github.com/lucasew/workspaced/pkg/driver/exec"
)

type Command struct {
	Binary    *Binary
	InStore   *InStore   `cmd:"in-store"`
	KnownNode *KnownNode `cmd:"known-node"`
	Node      *Node
	Phone     *Phone
}

func (Command) Description() string {
	return "Environment detection commands"
}

func (c *Command) Run(ctx context.Context) error {
	return clirun.PrintUsage[Command]("workspaced is")
}

type Binary struct {
	name cmd.StringArg
}

func (Binary) Description() string { return "Check if binary is available" }
func (b *Binary) Run(ctx context.Context) error {
	if !execdriver.IsBinaryAvailable(ctx, b.name.Value()) {
		return fmt.Errorf("binary %s not available", b.name.Value())
	}
	return nil
}

type Phone struct{}

func (Phone) Description() string { return "Check if environment is a phone" }
func (*Phone) Run(ctx context.Context) error {
	if !envdriver.IsPhone(ctx) {
		return ErrNotPhone
	}
	return nil
}
