package taskgroup

import (
	"context"
	"fmt"
	"sync/atomic"
)

// Map fans out Items under one aggregate Control progress bar.
//
// Fill the struct and call Run. Run schedules a Control orchestrator on the
// Group in ctx (MustFromContext), waits for every child, and returns results
// in input order. The orchestrator is the sole progress owner — do not wrap
// Run in another Control+Unit parent or call Unit on children unless an item
// has multi-step progress of its own.
type Map[T any, U any] struct {
	// Name is the orchestrator task label (and sole aggregate bar).
	// Empty defaults to "map".
	Name string
	// Items to process.
	Items []T
	// Pool selects the resource pool per item. When nil, PoolKind is used for
	// every item (zero value is Control).
	Pool func(T) PoolKind
	// PoolKind is used when Pool is nil.
	PoolKind PoolKind
	// TaskName labels each child task for logs/TUI. Nil uses "name:<i>".
	TaskName func(int, T) string
	// Fn runs for each item with its own *Status.
	Fn func(ctx context.Context, s *Status, item T) (U, error)
}

func (m Map[T, U]) poolFor(item T) PoolKind {
	if m.Pool != nil {
		return m.Pool(item)
	}
	return m.PoolKind
}

func (m Map[T, U]) childName(i int, item T, name string) string {
	if m.TaskName != nil {
		if n := m.TaskName(i, item); n != "" {
			return n
		}
	}
	return fmt.Sprintf("%s:%d", name, i)
}

// Run schedules the map on MustFromContext(ctx) and blocks until complete.
// Nested Run calls schedule on the group carried by ctx (typically inside a
// parent task). An empty Items list returns a non-nil empty slice.
func (m Map[T, U]) Run(ctx context.Context) ([]U, error) {
	if m.Fn == nil {
		return nil, fmt.Errorf("taskgroup: Map.Fn is nil")
	}
	if len(m.Items) == 0 {
		return []U{}, nil
	}

	name := m.Name
	if name == "" {
		name = "map"
	}

	parent := MustFromContext(ctx)
	total := int64(len(m.Items))

	var (
		results []U
		mapErr  error
	)
	done := make(chan struct{})

	// Control task owns aggregate progress; children live on a SubGroup so
	// they prune independently and do not bloat the parent's Live set forever.
	parent.Go(name, Control, func(ctx context.Context, s *Status) error {
		defer close(done)

		s.Update(fmt.Sprintf("0/%d", total))
		s.Progress(0, total)

		childGroup, _ := parent.SubGroup(ctx)
		results = make([]U, len(m.Items))
		var completed atomic.Int64

		for i := range m.Items {
			i := i
			item := m.Items[i]
			childName := m.childName(i, item, name)
			pool := m.poolFor(item)

			childGroup.Go(childName, pool, func(ctx context.Context, itemStatus *Status) error {
				u, err := m.Fn(ctx, itemStatus, item)
				if err != nil {
					return err
				}
				results[i] = u

				cur := completed.Add(1)
				s.Progress(cur, total)
				s.Update(fmt.Sprintf("%d/%d", cur, total))
				return nil
			})
		}

		mapErr = childGroup.Wait()
		if mapErr == nil {
			s.Progress(total, total)
			s.Update(fmt.Sprintf("%d/%d", total, total))
		}
		return mapErr
	})

	select {
	case <-done:
	case <-ctx.Done():
		// Control task observes cancellation via shared pools/context; still
		// wait for it so we do not leak the done channel waiter ordering.
		<-done
		if mapErr == nil {
			mapErr = ctx.Err()
		}
	}
	if mapErr != nil {
		return nil, mapErr
	}
	return results, nil
}
