package common

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"workspaced/pkg/logging"
)

var (
	cacheLockMu sync.Mutex
	cacheLocks  = map[string]*sync.Mutex{}
)

func EnsureCachedDir(provider string, key string, fetch func(tmpDir string) error) (string, error) {
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
	if st, err := os.Stat(dest); err == nil && st.IsDir() {
		slog.Debug("source cache hit", "provider", provider, "cache_dir", dest)
		return dest, nil
	}

	lock := keyLock(provider + "|" + key)
	lock.Lock()
	defer lock.Unlock()

	if st, err := os.Stat(dest); err == nil && st.IsDir() {
		slog.Debug("source cache hit after wait", "provider", provider, "cache_dir", dest)
		return dest, nil
	}

	slog.Info("source cache miss", "provider", provider, "cache_dir", dest)
	tmpDest := dest + ".tmp"
	logging.RunCleanup(context.Background(), "remove_all", func() error { return os.RemoveAll(tmpDest) }, slog.String("path", tmpDest))
	slog.Info("source fetch start", "provider", provider, "tmp_dir", tmpDest)
	if err := fetch(tmpDest); err != nil {
		logging.RunCleanup(context.Background(), "remove_all", func() error { return os.RemoveAll(tmpDest) }, slog.String("path", tmpDest))
		return "", err
	}
	if err := os.Rename(tmpDest, dest); err != nil {
		logging.RunCleanup(context.Background(), "remove_all", func() error { return os.RemoveAll(tmpDest) }, slog.String("path", tmpDest))
		return "", err
	}
	slog.Info("source fetch done", "provider", provider, "cache_dir", dest)
	return dest, nil
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
