package dotfiles

import (
	"context"
	"github.com/lucasew/workspaced/internal/deployer"
)

// Hook allows executing code before/after deployment.
type Hook interface {
	// Before is called before executing actions.
	Before(ctx context.Context, actions []deployer.Action) error

	// After is called after executing actions (even if there was an error).
	After(ctx context.Context, applied []deployer.Action, err error) error
}

// FuncHook implements Hook using functions.
type FuncHook struct {
	BeforeFn func(ctx context.Context, actions []deployer.Action) error
	AfterFn  func(ctx context.Context, applied []deployer.Action, err error) error
}

func (h *FuncHook) Before(ctx context.Context, actions []deployer.Action) error {
	if h.BeforeFn != nil {
		return h.BeforeFn(ctx, actions)
	}
	return nil
}

func (h *FuncHook) After(ctx context.Context, applied []deployer.Action, err error) error {
	if h.AfterFn != nil {
		return h.AfterFn(ctx, applied, err)
	}
	return nil
}
