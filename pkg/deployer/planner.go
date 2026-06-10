package deployer

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"strings"
	"workspaced/pkg/logging"
	"workspaced/pkg/source"
	"workspaced/pkg/taskgroup"
)

// Planner compara estado atual vs desejado e gera ações
type Planner struct{}

// NewPlanner cria um novo planner
func NewPlanner() *Planner {
	return &Planner{}
}

func calculateHash(r io.Reader) (string, error) {
	h := sha256.New()
	if _, err := io.Copy(h, r); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

func planOne(target string, d DesiredState, current ManagedInfo, managed bool) (Action, error) {
	info, err := os.Lstat(target)
	exists := err == nil

	if !exists {
		return Action{Type: ActionCreate, Target: target, Desired: d}, nil
	}

	// Bundle fast-path: if managed source fingerprint is identical, skip per-file hashing.
	// This is used by generator modules (e.g. icons) that already encode a bundle hash in SourceInfo.
	if managed && current.SourceInfo == d.File.SourceInfo() && strings.Contains(current.SourceInfo, "bundle:") {
		return Action{Type: ActionNoop, Target: target, Desired: d, Current: current}, nil
	}

	needsUpdate := false

	desiredIsSymlink := d.File.Type() == source.TypeSymlink
	actualIsSymlink := info.Mode()&os.ModeSymlink != 0

	if desiredIsSymlink != actualIsSymlink {
		needsUpdate = true
	} else if desiredIsSymlink {
		desiredTarget, err := d.File.LinkTarget()
		if err != nil {
			return Action{}, fmt.Errorf("failed to get desired link target for %s: %w", d.File.SourceInfo(), err)
		}
		actualTarget, err := os.Readlink(target)
		if err != nil || desiredTarget != actualTarget {
			needsUpdate = true
		}
	} else {
		if info.Mode().Perm() != d.File.Mode().Perm() {
			needsUpdate = true
		} else {
			reader, err := d.File.Reader()
			if err != nil {
				return Action{}, fmt.Errorf("failed to get reader for %s: %w", d.File.SourceInfo(), err)
			}
			desiredHash, err := calculateHash(reader)
			logging.Close(logging.NewRootContext(nil), reader)
			if err != nil {
				return Action{}, err
			}

			targetFile, err := os.Open(target)
			if err != nil {
				needsUpdate = true
			} else {
				actualHash, err := calculateHash(targetFile)
				logging.Close(logging.NewRootContext(nil), targetFile)
				if err != nil {
					return Action{}, err
				}
				if desiredHash != actualHash {
					needsUpdate = true
				}
			}
		}
	}

	if needsUpdate {
		return Action{Type: ActionUpdate, Target: target, Desired: d, Current: current}, nil
	}
	if !managed || current.SourceInfo != d.File.SourceInfo() {
		return Action{Type: ActionUpdate, Target: target, Desired: d, Current: current}, nil
	}
	return Action{Type: ActionNoop, Target: target, Desired: d, Current: current}, nil
}

// Plan compares desired state with current state and returns necessary actions.
func (p *Planner) Plan(ctx context.Context, desired []DesiredState, currentState *State) ([]Action, error) {
	actions := make([]Action, len(desired))
	desiredMap := make(map[string]DesiredState, len(desired))
	for _, d := range desired {
		desiredMap[d.Target()] = d
	}

	// A task group must be present in the context (provided by the top-level
	// command). We use a SubGroup so that this planning work participates in
	// the parent's pool limits and cancellation.
	parent := taskgroup.MustFromContext(ctx)
	g, ctx := parent.SubGroup(ctx)

	for i, d := range desired {
		idx := i
		ds := d
		g.Go(fmt.Sprintf("plan:%s", ds.Target()), taskgroup.CPU, func(ctx context.Context, s *taskgroup.Status) error {
			target := ds.Target()
			current, managed := currentState.Files[target]
			a, err := planOne(target, ds, current, managed)
			if err != nil {
				return err
			}
			actions[idx] = a
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return nil, err
	}

	// Prune orphaned files.
	for target, current := range currentState.Files {
		if _, ok := desiredMap[target]; !ok {
			actions = append(actions, Action{Type: ActionDelete, Target: target, Current: current})
		}
	}

	return actions, nil
}
