package dotfiles

import (
	"context"

	"workspaced/pkg/deployer"
	"workspaced/pkg/logging"
)

// LogApplyOptions controls how ApplyResult is reported after plan/apply.
type LogApplyOptions struct {
	// ShowNoop includes no-op actions in the per-action log and summary count.
	ShowNoop bool
	// DryRun is true for plan runs. When set with NoChangesTarget, suppresses
	// the idle "no changes needed" line (plan output is action lines only).
	DryRun bool
	// NoChangesTarget, if non-empty, logs "no changes needed" with that target
	// when the result has nothing to report and DryRun is false. Home apply
	// leaves this empty (Manager already logs idle applies); codebase apply
	// sets it to "repo root".
	NoChangesTarget string
}

// LogApplyResult writes per-action lines and a summary for an ApplyResult.
// Safe to call with a nil result (no-op). Used by home and codebase plan/apply
// so reporting stays in one place.
func LogApplyResult(ctx context.Context, result *ApplyResult, opts LogApplyOptions) {
	if result == nil {
		return
	}
	logger := logging.GetLogger(ctx)
	hasChanges := result.FilesCreated > 0 || result.FilesUpdated > 0 || result.FilesDeleted > 0 || (opts.ShowNoop && result.FilesNoOp > 0)
	if !hasChanges {
		if opts.NoChangesTarget != "" && !opts.DryRun {
			logger.Info("no changes needed", "target", opts.NoChangesTarget)
		}
		return
	}
	for _, a := range deployer.SortActions(result.Actions) {
		if a.Type == deployer.ActionNoop && !opts.ShowNoop {
			continue
		}
		sourceInfo := ""
		if a.Desired.File != nil {
			sourceInfo = a.Desired.File.SourceInfo()
		}
		logger.Info("apply action",
			"type", a.Type,
			"target", deployer.PrettyPath(a.Target),
			"source", sourceInfo,
		)
	}
	attrs := []any{
		"created", result.FilesCreated,
		"updated", result.FilesUpdated,
		"deleted", result.FilesDeleted,
	}
	if opts.ShowNoop {
		attrs = append(attrs, "noop", result.FilesNoOp)
	}
	logger.Info("apply summary", attrs...)
}
