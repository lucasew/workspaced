package rsync

import (
	"context"
	"errors"
	"io"
)

var (
	ErrNeedsSrcAndDst = errors.New("rsync requires src and dst")
)

// Options configures an rsync transfer.
type Options struct {
	// Excludes are patterns to pass as --exclude=...
	Excludes []string
	// SkipPermissions adds --no-perms (useful for some remote filesystems).
	SkipPermissions bool
	// Output, if non-nil, receives a combined transcript of rsync stdout+stderr
	// (one line per update). Useful for live UIs outside of taskgroup.
	Output io.Writer
}

// Driver performs rsync-style transfers.
// Implementations are selected via weights (native preferred over pure-Go fallback).
type Driver interface {
	// Sync copies files from src to dst using rsync semantics.
	// Both src and dst may be local paths or remote rsync URLs (user@host:path, rsync://...).
	//
	// When a taskgroup.Group is present in ctx (via taskgroup.FromContext), Sync
	// schedules the actual transfer as a child task ("rsync:...") in the IO pool
	// so that progress is tracked and rendered by the taskgroup system (Update + Progress).
	//
	// Progress is driven from rsync output lines (current file / stats). The total
	// is often indeterminate (-1) unless size information can be extracted.
	Sync(ctx context.Context, src, dst string, opts Options) error
}
