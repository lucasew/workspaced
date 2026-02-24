package deployer

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"
	"sync"
	"workspaced/pkg/source"
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
			_ = reader.Close()
			if err != nil {
				return Action{}, err
			}

			targetFile, err := os.Open(target)
			if err != nil {
				needsUpdate = true
			} else {
				actualHash, err := calculateHash(targetFile)
				_ = targetFile.Close()
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

// Plan compara desired state com current state e retorna ações necessárias
func (p *Planner) Plan(ctx context.Context, desired []DesiredState, currentState *State) ([]Action, error) {
	actions := make([]Action, len(desired))
	desiredMap := make(map[string]DesiredState, len(desired))
	for _, d := range desired {
		desiredMap[d.Target()] = d
	}

	type task struct {
		idx int
		d   DesiredState
	}
	tasks := make(chan task)
	errCh := make(chan error, 1)
	var wg sync.WaitGroup

	workers := runtime.NumCPU()
	if workers < 1 {
		workers = 1
	}
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for t := range tasks {
				target := t.d.Target()
				current, managed := currentState.Files[target]
				a, err := planOne(target, t.d, current, managed)
				if err != nil {
					select {
					case errCh <- err:
					default:
					}
					return
				}
				actions[t.idx] = a
			}
		}()
	}

	for i, d := range desired {
		select {
		case err := <-errCh:
			close(tasks)
			wg.Wait()
			return nil, err
		default:
		}
		tasks <- task{idx: i, d: d}
	}
	close(tasks)
	wg.Wait()
	select {
	case err := <-errCh:
		return nil, err
	default:
	}

	// Prune orphaned files
	for target, current := range currentState.Files {
		if _, ok := desiredMap[target]; !ok {
			actions = append(actions, Action{Type: ActionDelete, Target: target, Current: current})
		}
	}

	return actions, nil
}
