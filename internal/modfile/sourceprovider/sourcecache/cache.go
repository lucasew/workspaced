package sourcecache

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"sync"

	"github.com/lucasew/workspaced/internal/cmdctx"
	"github.com/lucasew/workspaced/pkg/logging"
)

var (
	cacheLockMu sync.Mutex
	cacheLocks  = map[string]*sync.Mutex{}
)

func EnsureCachedDir(ctx context.Context, provider string, key string, fetch func(tmpDir string) error) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	cacheRoot := filepath.Join(home, ".cache", "workspaced", "sources", provider)
	if err := os.MkdirAll(cacheRoot, 0755); err != nil {
		return "", err
	}

	hash := sha256.Sum256([]byte(key))
	dest := filepath.Join(cacheRoot, hex.EncodeToString(hash[:]))
	logger := logging.GetLogger(ctx)
	noCache := cmdctx.IsNoCache(ctx)

	if st, err := os.Stat(dest); err == nil && st.IsDir() && !noCache {
		logger.Debug("source cache hit", "provider", provider, "cache_dir", dest)
		return dest, nil
	}

	lock := keyLock(provider + "|" + key)
	lock.Lock()
	defer lock.Unlock()

	if st, err := os.Stat(dest); err == nil && st.IsDir() && !noCache {
		logger.Debug("source cache hit after wait", "provider", provider, "cache_dir", dest)
		return dest, nil
	}

	// Dry-run + no-cache: widen plan only; do not re-fetch.
	if noCache && cmdctx.IsDryRun(ctx) {
		if st, err := os.Stat(dest); err == nil && st.IsDir() {
			logger.Debug("no-cache: would re-fetch source (dry-run)", "provider", provider, "cache_dir", dest)
			return dest, nil
		}
	}

	if noCache {
		logger.Debug("no-cache: source cache miss", "provider", provider, "cache_dir", dest)
	} else {
		logger.Info("source cache miss", "provider", provider, "cache_dir", dest)
	}
	tmpDest := dest + ".tmp"
	logging.RunCleanup(ctx, "remove_all", func() error { return os.RemoveAll(tmpDest) }, "path", tmpDest)
	if err := os.RemoveAll(tmpDest); err != nil {
		return "", err
	}
	if err := os.MkdirAll(tmpDest, 0755); err != nil {
		return "", err
	}
	logger.Info("source fetch start", "provider", provider, "tmp_dir", tmpDest)
	if err := fetch(tmpDest); err != nil {
		logging.RunCleanup(ctx, "remove_all", func() error { return os.RemoveAll(tmpDest) }, "path", tmpDest)
		return "", err
	}
	if err := atomicReplaceDir(dest, tmpDest); err != nil {
		logging.RunCleanup(ctx, "remove_all", func() error { return os.RemoveAll(tmpDest) }, "path", tmpDest)
		return "", err
	}
	logger.Info("source fetch done", "provider", provider, "cache_dir", dest)
	return dest, nil
}

// atomicReplaceDir moves tmpDir into place at dest, replacing any existing dest.
func atomicReplaceDir(dest, tmpDir string) error {
	old := dest + ".old"
	_ = os.RemoveAll(old)
	if _, err := os.Stat(dest); err == nil {
		if err := os.Rename(dest, old); err != nil {
			if err := os.RemoveAll(dest); err != nil {
				return err
			}
		}
	}
	if err := os.Rename(tmpDir, dest); err != nil {
		_ = os.Rename(old, dest)
		return err
	}
	_ = os.RemoveAll(old)
	return nil
}

func keyLock(key string) *sync.Mutex {
	cacheLockMu.Lock()
	defer cacheLockMu.Unlock()
	lock, ok := cacheLocks[key]
	if ok {
		return lock
	}
	lock = &sync.Mutex{}
	cacheLocks[key] = lock
	return lock
}
