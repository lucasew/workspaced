package deployer

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"workspaced/pkg/logging"
	"workspaced/pkg/source"
	"workspaced/pkg/taskgroup"
)

// PrettyPath converts an absolute path to a path relative to $HOME.
func PrettyPath(path string) string {
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
// When a taskgroup.Group is present in ctx, actions are applied concurrently
// (bounded by the IO pool) using Map. State mutations are protected by a mutex.
func (e *Executor) Execute(ctx context.Context, actions []Action, state *State) error {
	logger := logging.GetLogger(ctx)
	orderedActions := SortActions(actions)

	// Collect non-noop work items.
	work := make([]Action, 0, len(orderedActions))
	for _, a := range orderedActions {
		if a.Type != ActionNoop {
			work = append(work, a)
		}
	}
	if len(work) == 0 {
		return nil
	}

	var stateMu sync.Mutex

	// applyOne performs the filesystem work for a single action and updates
	// the state map under the mutex (safe for concurrent use).
	applyOne := func(ctx context.Context, action Action) error {
		switch action.Type {
		case ActionDelete:
			logger.Info("pruning orphaned file", "target", PrettyPath(action.Target))
			if _, err := os.Lstat(action.Target); err == nil {
				if err := os.Remove(action.Target); err != nil {
					return fmt.Errorf("failed to remove orphaned file %s: %w", action.Target, err)
				}
			}
			stateMu.Lock()
			delete(state.Files, action.Target)
			stateMu.Unlock()
			return nil

		case ActionCreate, ActionUpdate:
			if action.Type == ActionCreate {
				logger.Info("creating", "target", PrettyPath(action.Target), "source", action.Desired.File.SourceInfo())
			} else {
				logger.Info("updating", "target", PrettyPath(action.Target), "source", action.Desired.File.SourceInfo())
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
				stateMu.Lock()
				state.Files[action.Target] = ManagedInfo{SourceInfo: action.Desired.File.SourceInfo()}
				stateMu.Unlock()
				return nil
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

			stateMu.Lock()
			state.Files[action.Target] = ManagedInfo{SourceInfo: action.Desired.File.SourceInfo()}
			stateMu.Unlock()
			return nil
		}
		return nil
	}

	// If we have a task group in context, run the actions in parallel
	// using the IO pool. Each action gets its own task + Status.
	if taskgroup.FromContext(ctx) != nil {
		_, err := taskgroup.Map(ctx, func(Action) taskgroup.PoolKind { return taskgroup.IO }, work,
			func(_ int, a Action) string {
				p := PrettyPath(a.Target)
				switch a.Type {
				case ActionCreate:
					return "create:" + p
				case ActionUpdate:
					return "update:" + p
				case ActionDelete:
					return "delete:" + p
				default:
					return "apply:" + p
				}
			},
			func(ctx context.Context, s *taskgroup.Status, a Action) (struct{}, error) {
				s.Progress(0, 1)
				if err := applyOne(ctx, a); err != nil {
					return struct{}{}, err
				}
				s.Progress(1, 1)
				return struct{}{}, nil
			})
		return err
	}

	// Sequential fallback (no taskgroup in ctx).
	for _, a := range work {
		if err := applyOne(ctx, a); err != nil {
			return err
		}
	}
	return nil
}

// compile-time check that taskgroup is imported
var _ = taskgroup.IO
