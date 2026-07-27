package apply

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/lucasew/workspaced/pkg/logging"
)

func TestWriteTempDconfIni_UniqueAndContents(t *testing.T) {
	ctx := logging.NewWriterContext(t.Output())
	const body = "[org/gnome/desktop/interface]\ncolor-scheme='prefer-dark'\n\n"
	p1, err := writeTempDconfIni(ctx, body)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Remove(p1); err != nil {
			t.Errorf("remove %s: %v", p1, err)
		}
	})

	p2, err := writeTempDconfIni(ctx, body)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Remove(p2); err != nil {
			t.Errorf("remove %s: %v", p2, err)
		}
	})

	if p1 == p2 {
		t.Fatalf("expected unique temp paths, both %q", p1)
	}
	if filepath.Base(p1) == "workspaced-dconf.ini" {
		t.Fatalf("still using fixed temp name: %q", p1)
	}
	if !strings.Contains(filepath.Base(p1), "workspaced-dconf-") {
		t.Fatalf("unexpected temp basename: %q", filepath.Base(p1))
	}

	got, err := os.ReadFile(p1)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != body {
		t.Fatalf("content mismatch:\n got %q\nwant %q", got, body)
	}

	info, err := os.Stat(p1)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm()&0o077 != 0 {
		t.Fatalf("temp file should not be group/other accessible: mode %o", info.Mode().Perm())
	}
}

func TestWriteTempDconfIni_ConcurrentNoCollision(t *testing.T) {
	ctx := logging.NewWriterContext(t.Output())
	const n = 16
	paths := make([]string, n)
	var wg sync.WaitGroup
	wg.Add(n)
	errCh := make(chan error, n)
	for i := 0; i < n; i++ {
		i := i
		go func() {
			defer wg.Done()
			p, err := writeTempDconfIni(ctx, strings.Repeat("x", i+1))
			if err != nil {
				errCh <- err
				return
			}
			paths[i] = p
		}()
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		for _, p := range paths {
			if p != "" {
				if err := os.Remove(p); err != nil {
					t.Errorf("remove %s: %v", p, err)
				}
			}
		}
	})
	seen := map[string]struct{}{}
	for _, p := range paths {
		if p == "" {
			t.Fatal("empty path from concurrent write")
		}
		if _, ok := seen[p]; ok {
			t.Fatalf("duplicate temp path under concurrency: %q", p)
		}
		seen[p] = struct{}{}
	}
}
