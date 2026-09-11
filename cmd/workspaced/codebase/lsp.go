package codebase

import (
	"context"
	"fmt"
	"os"

	"github.com/lucasew/workspaced/internal/lsp"
	"github.com/lucasew/workspaced/pkg/logging"
)

type Lsp struct{}

func (Lsp) Description() string {
	return "Experimental: language server router (stdio LSP proxy driven by workspaced.cue)"
}

func (*Lsp) Run(ctx context.Context) error {
	logger := logging.GetLogger(ctx)
	logger.Info("codebase lsp starting (stdio)")
	err := lsp.Run(ctx, os.Stdin, os.Stdout)
	if err != nil {
		return fmt.Errorf("lsp: %w", err)
	}
	return nil
}
