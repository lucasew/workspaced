package backup

import (
	"context"
	"workspaced/pkg/cmdctx"
)

func verboseOutput(ctx context.Context) bool {
	return cmdctx.IsVerbose(ctx)
}
