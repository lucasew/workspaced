package input

import (
	"context"
	"errors"
	"fmt"

	"github.com/lewtec/lewkit/x/cmd"
	"github.com/lucasew/workspaced/internal/clirun"
	"github.com/lucasew/workspaced/pkg/driver/dialog"
)

var ErrCancelled = errors.New("cancelled")

type Command struct {
	Text      *Text
	Confirm   *Confirm
	Choose    *Choose
	Launch    *Launch
	Window    *Window
	Workspace *Workspace
}

func (Command) Description() string {
	return "Interactive user input commands"
}

func (c *Command) Run(ctx context.Context) error {
	return clirun.PrintUsage[Command]("workspaced driver input")
}

type Text struct {
	prompt *cmd.StringArg
}

func (Text) Description() string { return "Ask for a text string" }

func (c *Text) Run(ctx context.Context) error {
	prompt := "Input"
	if c.prompt != nil {
		prompt = c.prompt.Value()
	}
	res, err := dialog.Prompt(ctx, prompt)
	if err != nil {
		return err
	}
	fmt.Println(res)
	return nil
}

type Confirm struct {
	message *cmd.StringArg
}

func (Confirm) Description() string { return "Ask for a yes/no confirmation" }

func (c *Confirm) Run(ctx context.Context) error {
	msg := "Confirm?"
	if c.message != nil {
		msg = c.message.Value()
	}
	ok, err := dialog.Confirm(ctx, msg)
	if err != nil {
		return err
	}
	if !ok {
		return ErrCancelled
	}
	return nil
}

type Choose struct {
	prompt  cmd.StringArg
	options []cmd.StringArg
}

func (Choose) Description() string { return "Select an item from a list" }

func (c *Choose) Run(ctx context.Context) error {
	items := make([]dialog.Item, 0, len(c.options))
	for _, arg := range c.options {
		items = append(items, dialog.Item{Label: arg.Value(), Value: arg.Value()})
	}
	res, err := dialog.Choose(ctx, dialog.ChooseOptions{
		Prompt: c.prompt.Value(),
		Items:  items,
	})
	if err != nil {
		return err
	}
	if res != nil {
		fmt.Println(res.Value)
	}
	return nil
}
