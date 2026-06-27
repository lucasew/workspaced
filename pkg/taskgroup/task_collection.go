package taskgroup

// TaskCollection is the live-task set for a Group: O(1) insert/delete by id,
// O(1) dep resolution by description (latest live task with that label), and
// waiter-aware pruning so completed tasks do not linger for Snapshot/TUI.
//
// Not safe for concurrent use; the owning Group serializes access with its mu.
type TaskCollection struct {
	// ByID is the set of live (not yet pruned) tasks, keyed by UUIDv7.
	ByID map[string]*taskEntry
	// LatestByDesc is the most recently Add()'d live task per description,
	// so dep resolution by name stays O(1) without scanning ByID.
	LatestByDesc map[string]*taskEntry
}

// NewTaskCollection returns an empty live-task set.
func NewTaskCollection() TaskCollection {
	return TaskCollection{
		ByID:         make(map[string]*taskEntry),
		LatestByDesc: make(map[string]*taskEntry),
	}
}

// Add registers a newly scheduled task as live and marks it as the latest
// task for its description.
func (c *TaskCollection) Add(t *taskEntry) {
	c.ByID[t.id] = t
	c.LatestByDesc[t.desc] = t
}

// Lookup resolves a dependency key: exact task id, else the latest live task
// with that description.
func (c *TaskCollection) Lookup(dep string) *taskEntry {
	if t, ok := c.ByID[dep]; ok {
		return t
	}
	if t, ok := c.LatestByDesc[dep]; ok {
		return t
	}
	return nil
}

// PinDeps resolves each dep name against the live set, increments waiter
// counts, and stores the pins on t.resolvedDeps. Call before or after Add(t);
// deps must already be live (scheduled earlier).
func (c *TaskCollection) PinDeps(t *taskEntry, deps []string) {
	if len(deps) == 0 {
		return
	}
	t.resolvedDeps = make([]*taskEntry, 0, len(deps))
	for _, dep := range deps {
		if depTask := c.Lookup(dep); depTask != nil {
			depTask.waiters++
			t.resolvedDeps = append(t.resolvedDeps, depTask)
		}
	}
}

// ReleaseAndPrune decrements waiter pins on t's resolved deps and removes any
// terminal tasks that no longer have waiters (deps first, then t).
func (c *TaskCollection) ReleaseAndPrune(t *taskEntry) {
	for _, dep := range t.resolvedDeps {
		if dep.waiters > 0 {
			dep.waiters--
		}
		c.Prune(dep)
	}
	c.Prune(t)
}

// Prune removes a terminal task with no remaining waiters from the live set.
func (c *TaskCollection) Prune(t *taskEntry) {
	if t == nil {
		return
	}
	t.mu.Lock()
	st := t.state
	t.mu.Unlock()
	if st != Done && st != Failed {
		return
	}
	if t.waiters > 0 {
		return
	}
	if _, ok := c.ByID[t.id]; !ok {
		return
	}
	delete(c.ByID, t.id)
	if c.LatestByDesc[t.desc] == t {
		delete(c.LatestByDesc, t.desc)
		// Restore latest among remaining live tasks with the same description.
		// UUIDv7 ids are time-ordered, so max id ≈ most recently scheduled.
		var best *taskEntry
		for _, cur := range c.ByID {
			if cur.desc != t.desc {
				continue
			}
			if best == nil || cur.id > best.id {
				best = cur
			}
		}
		if best != nil {
			c.LatestByDesc[t.desc] = best
		}
	}
}

// ForEach visits every live task (map iteration order).
func (c *TaskCollection) ForEach(fn func(*taskEntry)) {
	for _, t := range c.ByID {
		fn(t)
	}
}

// Entries returns a snapshot of live task pointers for use outside the lock.
func (c *TaskCollection) Entries() []*taskEntry {
	out := make([]*taskEntry, 0, len(c.ByID))
	for _, t := range c.ByID {
		out = append(out, t)
	}
	return out
}

// Len returns the number of live tasks.
func (c *TaskCollection) Len() int {
	return len(c.ByID)
}
