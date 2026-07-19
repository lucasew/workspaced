package codebase

import (
	"fmt"
	"os"

	"workspaced/internal/lsp"
	"workspaced/pkg/logging"

	"github.com/spf13/cobra"
)

func init() {
	Registry.Register(func(c *cobra.Command) {
		c.AddCommand(&cobra.Command{
			Use:   "lsp",
			Short: "Experimental: language server router (stdio LSP proxy driven by workspaced.cue)",
			Long: `Experimental. Speak LSP on stdio and route to language servers declared in codebase workspaced.cue.

API, cue schema, and merge behavior may change without a migration path.

The editor should use this as its only language server. On initialize, the
client root is taken from rootUri (single workspace folder only). Config is
loaded from that root's workspaced.cue lsp block:

  workspaced: {
    lsp: {
      extensions: { ".go": "go" }
      language_ids: { go: "go", gomod: "go" }
      languages: {
        go: {
          "00_gopls": { capabilities: { hover: true, definition: true, completion: true, diagnostics: true } }
        }
      }
      servers: {
        gopls: {
          cmd: ["gopls"]
          needs: { gopls: true } // optional lazy_tools names to ensure first
        }
      }
    }
    lazy_tools: {
      gopls: { ref: "..." }
    }
  }

An empty or missing lsp block still accepts the connection; document methods
return unsupported until routes exist.

Stdout is the LSP wire. Logs go to stderr.`,
			RunE: func(cmd *cobra.Command, args []string) error {
				ctx := cmd.Context()
				logger := logging.GetLogger(ctx)
				logger.Info("codebase lsp starting (stdio)")
				err := lsp.Run(ctx, os.Stdin, os.Stdout)
				if err != nil {
					return fmt.Errorf("lsp: %w", err)
				}
				return nil
			},
		})
	})
}
