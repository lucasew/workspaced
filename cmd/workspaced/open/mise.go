package open

import (
	"context"

	"github.com/lewtec/lewkit/x/cmd"
)

type Mise struct {
	sep  cmd.Dash
	args []cmd.StringArg
}

func (Mise) Description() string {
	return `Alias for: open lazy --home mise -- …

Convenience alias for the standard home lazy tool path:

  workspaced open lazy --home --bin mise mise -- [args...]

mise is declared as lazy_tools.mise (registry:mise) and installed into the
tool store like any other catalog tool. The lockfile pins the version.
Package installs via the mise: backend still shell out to that binary.

Examples:
  workspaced open mise -- version
  workspaced open mise -- install node@20
  workspaced open lazy --home mise -- version`
}

func (m *Mise) Run(ctx context.Context) error {
	return runLazyTool(ctx, true, "mise", "mise", cmd.Values(m.args))
}
