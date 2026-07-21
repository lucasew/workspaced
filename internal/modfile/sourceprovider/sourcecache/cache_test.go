package sourcecache

import (
	"context"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/lucasew/workspaced/internal/cmdctx"
	"github.com/lucasew/workspaced/pkg/logging"
)

func testCtx(t *testing.T) context.Context {
	t.Helper()
	return logging.NewWriterContext(t.Output())
}

func TestEnsureCachedDirHitAndNoCache(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	// UserHomeDir on some platforms also checks these; keep cache under home.
	t.Setenv("XDG_CACHE_HOME", filepath.Join(home, ".cache"))

	ctx := testCtx(t)
	var fetches atomic.Int32
	fetch := func(tmpDir string) error {
		fetches.Add(1)
		return os.WriteFile(filepath.Join(tmpDir, "marker"), []byte("v1"), 0o644)
	}

	dir1, err := EnsureCachedDir(ctx, "test", "key-a", fetch)
	if err != nil {
		t.Fatalf("first ensure: %v", err)
	}
	if fetches.Load() != 1 {
		t.Fatalf("fetches after miss = %d, want 1", fetches.Load())
	}

	dir2, err := EnsureCachedDir(ctx, "test", "key-a", fetch)
	if err != nil {
		t.Fatalf("second ensure: %v", err)
	}
	if dir1 != dir2 {
		t.Fatalf("cache dir changed: %q vs %q", dir1, dir2)
	}
	if fetches.Load() != 1 {
		t.Fatalf("fetches after hit = %d, want 1", fetches.Load())
	}

	// no-cache: re-fetch even though warm
	ctxNo := cmdctx.WithNoCache(ctx, true)
	dir3, err := EnsureCachedDir(ctxNo, "test", "key-a", fetch)
	if err != nil {
		t.Fatalf("no-cache ensure: %v", err)
	}
	if dir3 != dir1 {
		t.Fatalf("dest path should be stable, got %q want %q", dir3, dir1)
	}
	if fetches.Load() != 2 {
		t.Fatalf("fetches after no-cache = %d, want 2", fetches.Load())
	}

	// no-cache + dry-run: do not re-fetch when warm
	ctxPlan := cmdctx.WithDryRun(ctxNo, true)
	_, err = EnsureCachedDir(ctxPlan, "test", "key-a", fetch)
	if err != nil {
		t.Fatalf("dry-run no-cache: %v", err)
	}
	if fetches.Load() != 2 {
		t.Fatalf("fetches after dry-run no-cache = %d, want 2", fetches.Load())
	}
}
