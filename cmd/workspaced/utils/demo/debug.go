package demo

import (
	"context"
	"fmt"

	"github.com/lewtec/lewkit/x/cmd"
)

type Debug struct {
	Test cmd.StringArg `long:"test" help:"a test flag" default:"default"`
	args []cmd.StringArg
}

func (Debug) Description() string { return "Debug flag passing" }

func (d *Debug) Run(ctx context.Context) error {
	fmt.Printf("test flag value: %s\n", d.Test.Value())
	fmt.Printf("args: %v\n", cmd.Values(d.args))
	return nil
}
