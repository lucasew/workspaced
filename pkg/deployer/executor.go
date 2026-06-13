package deployer

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"workspaced/pkg/logging"
	"workspaced/pkg/source"
	"workspaced/pkg/taskgroup"
)

// prettyPath converts an absolute path to a path relative to $HOME.
func prettyPath(path string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}

	if after, ok := strings.CutPrefix(path, home+"/"); ok {
		return "~/" + after
	}

	return path
}

// Executor applies deployment actions.
type Executor struct{}

// NewExecutor creates a new Executor.
func NewExecutor() *Executor {
	return &Executor{}
}

// Execute applies a list of actions and updates state.
// If a taskgroup.Group is available in ctx, progress is reported via Status.
func (e *Executor) Execute(ctx context.Context, actions []Action, state *State) error {
	logger := logging.GetLogger(ctx)
	orderedActions := SortActions(actions)

	// Get the status handle if we're inside a taskgroup task.
	// The caller may wrap this in a taskgroup.Go call.

	total := int64(0)
	for _, a := range orderedActions {
		if a.Type != ActionNoop {
			total++
		}
	}

	var done int64
	for _, action := range orderedActions {
		switch action.Type {
		case ActionNoop:
			continue

		case ActionDelete:
			logger.Info("pruning orphaned file", "target", prettyPath(action.Target))
			if _, err := os.Lstat(action.Target); err == nil {
				if err := os.Remove(action.Target); err != nil {
					return fmt.Errorf("failed to remove orphaned file %s: %w", action.Target, err)
				}
			}
			delete(state.Files, action.Target)

		case ActionCreate, ActionUpdate:
			if action.Type == ActionCreate {
				logger.Info("creating", "target", prettyPath(action.Target), "source", action.Desired.File.SourceInfo())
			} else {
				logger.Info("updating", "target", prettyPath(action.Target), "source", action.Desired.File.SourceInfo())
			}

			if err := os.MkdirAll(filepath.Dir(action.Target), 0755); err != nil {
				return fmt.Errorf("failed to create parent directory for %s: %w", action.Target, err)
			}

			if _, err := os.Lstat(action.Target); err == nil {
				if err := os.RemoveAll(action.Target); err != nil {
					return fmt.Errorf("failed to remove existing target %s: %w", action.Target, err)
				}
			}

			if action.Desired.File.Type() == source.TypeSymlink {
				linkTarget, err := action.Desired.File.LinkTarget()
				if err != nil {
					return fmt.Errorf("failed to get link target for %s: %w", action.Desired.File.SourceInfo(), err)
				}
				if err := os.Symlink(linkTarget, action.Target); err != nil {
					return fmt.Errorf("failed to create symlink %s -> %s: %w", action.Target, linkTarget, err)
				}
				state.Files[action.Target] = ManagedInfo{SourceInfo: action.Desired.File.SourceInfo()}
				done++
				continue
			}

			f, err := os.OpenFile(action.Target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, action.Desired.File.Mode())
			if err != nil {
				return fmt.Errorf("failed to open target file %s: %w", action.Target, err)
			}

			reader, err := action.Desired.File.Reader()
			if err != nil {
				logging.Close(ctx, f)
				return fmt.Errorf("failed to get reader for %s: %w", action.Desired.File.SourceInfo(), err)
			}

			_, err = io.Copy(f, reader)
			logging.Close(ctx, reader)
			logging.Close(ctx, f)

			if err != nil {
				return fmt.Errorf("failed to write content to %s: %w", action.Target, err)
			}

			state.Files[action.Target] = ManagedInfo{SourceInfo: action.Desired.File.SourceInfo()}
			done++
		}
	}

	return nil
}

// compile-time check that taskgroup is imported
var _ = taskgroup.IO
