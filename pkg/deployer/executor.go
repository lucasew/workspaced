package deployer

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	envdriver "workspaced/pkg/driver/env"
	"workspaced/pkg/logging"
	"workspaced/pkg/source"
	"workspaced/pkg/taskgroup"
)

// RelToRoot returns path relative to root when path is under root.
// Otherwise returns path unchanged. Empty root is a no-op.
func RelToRoot(path, root string) string {
	path = strings.TrimSpace(path)
	root = strings.TrimSpace(root)
	if path == "" || root == "" {
		return path
	}
	root = filepath.Clean(root)
	path = filepath.Clean(path)
	rel, err := filepath.Rel(root, path)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return path
	}
	return rel
}

// AbsFromRoot resolves a state/display path against root.
// Absolute paths and "~/..." (via ExpandPath) are accepted for migration
// and display formats; other relative paths are joined to root.
func AbsFromRoot(path, root string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return path
	}
	expanded := envdriver.ExpandPath(path)
	if filepath.IsAbs(expanded) {
		return filepath.Clean(expanded)
	}
	root = strings.TrimSpace(root)
	if root == "" {
		return filepath.Clean(expanded)
	}
	return filepath.Clean(filepath.Join(root, expanded))
}

// PrettyPath converts an absolute path to a path relative to $HOME.
func PrettyPath(path string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	rel := RelToRoot(path, home)
	if rel == path {
		return path
	}
	if rel == "." {
		return "~"
	}
	return "~/" + filepath.ToSlash(rel)
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
		_, err := taskgroup.Map[Action, struct{}]{
			Name:     "apply",
			Items:    work,
			PoolKind: taskgroup.IO,
			TaskName: func(_ int, a Action) string {
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
			Fn: func(ctx context.Context, s *taskgroup.Status, a Action) (struct{}, error) {
				s.Update(PrettyPath(a.Target))
				if err := applyOne(ctx, a); err != nil {
					return struct{}{}, err
				}
				return struct{}{}, nil
			},
		}.Run(ctx)
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
