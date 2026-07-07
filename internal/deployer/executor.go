package deployer

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"workspaced/internal/source"
	envdriver "workspaced/pkg/driver/env"
	"workspaced/pkg/logging"
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

// statePatch is the pure state delta produced by applying one action's filesystem work.
type statePatch struct {
	delete bool
	target string
	info   ManagedInfo
}

func applyPatch(state *State, p statePatch) {
	if state == nil {
		return
	}
	if p.delete {
		delete(state.Files, p.target)
		return
	}
	state.Files[p.target] = p.info
}

// Execute applies a list of actions and updates state.
// With a taskgroup in ctx, filesystem work is mapped in parallel; state patches
// are reduced in input order afterward (no mutex on the live state map).
func (e *Executor) Execute(ctx context.Context, actions []Action, state *State) error {
	logger := logging.GetLogger(ctx)
	orderedActions := SortActions(actions)

	work := make([]Action, 0, len(orderedActions))
	for _, a := range orderedActions {
		if a.Type != ActionNoop {
			work = append(work, a)
		}
	}
	if len(work) == 0 {
		return nil
	}

	applyFS := func(ctx context.Context, action Action) (statePatch, error) {
		switch action.Type {
		case ActionDelete:
			logger.Info("pruning orphaned file", "target", PrettyPath(action.Target))
			if _, err := os.Lstat(action.Target); err == nil {
				if err := os.Remove(action.Target); err != nil {
					return statePatch{}, fmt.Errorf("failed to remove orphaned file %s: %w", action.Target, err)
				}
			}
			return statePatch{delete: true, target: action.Target}, nil

		case ActionCreate, ActionUpdate:
			if action.Type == ActionCreate {
				logger.Info("creating", "target", PrettyPath(action.Target), "source", action.Desired.File.SourceInfo())
			} else {
				logger.Info("updating", "target", PrettyPath(action.Target), "source", action.Desired.File.SourceInfo())
			}

			if err := os.MkdirAll(filepath.Dir(action.Target), 0755); err != nil {
				return statePatch{}, fmt.Errorf("failed to create parent directory for %s: %w", action.Target, err)
			}

			if _, err := os.Lstat(action.Target); err == nil {
				if err := os.RemoveAll(action.Target); err != nil {
					return statePatch{}, fmt.Errorf("failed to remove existing target %s: %w", action.Target, err)
				}
			}

			info := ManagedInfo{SourceInfo: action.Desired.File.SourceInfo()}
			if action.Desired.File.Type() == source.TypeSymlink {
				linkTarget, err := action.Desired.File.LinkTarget()
				if err != nil {
					return statePatch{}, fmt.Errorf("failed to get link target for %s: %w", action.Desired.File.SourceInfo(), err)
				}
				if err := os.Symlink(linkTarget, action.Target); err != nil {
					return statePatch{}, fmt.Errorf("failed to create symlink %s -> %s: %w", action.Target, linkTarget, err)
				}
				return statePatch{target: action.Target, info: info}, nil
			}

			f, err := os.OpenFile(action.Target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, action.Desired.File.Mode())
			if err != nil {
				return statePatch{}, fmt.Errorf("failed to open target file %s: %w", action.Target, err)
			}

			reader, err := action.Desired.File.Reader()
			if err != nil {
				logging.Close(ctx, f)
				return statePatch{}, fmt.Errorf("failed to get reader for %s: %w", action.Desired.File.SourceInfo(), err)
			}

			_, err = io.Copy(f, reader)
			logging.Close(ctx, reader)
			logging.Close(ctx, f)
			if err != nil {
				return statePatch{}, fmt.Errorf("failed to write content to %s: %w", action.Target, err)
			}
			return statePatch{target: action.Target, info: info}, nil
		}
		return statePatch{}, nil
	}

	taskName := func(_ int, a Action) string {
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
	}

	patches, err := taskgroup.Map[Action, statePatch]{
		Name:     "apply",
		Items:    work,
		PoolKind: taskgroup.IO,
		TaskName: taskName,
		Fn: func(ctx context.Context, s *taskgroup.Status, a Action) (statePatch, error) {
			s.Update(PrettyPath(a.Target))
			return applyFS(ctx, a)
		},
	}.Run(ctx)
	if err != nil {
		return err
	}
	for _, p := range patches {
		applyPatch(state, p)
	}
	return nil
}
