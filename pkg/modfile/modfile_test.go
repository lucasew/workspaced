package modfile

import (
	"path/filepath"
	"testing"
)

func TestResolveModuleSourceWithLockVersion(t *testing.T) {
	t.Parallel()

	mod := &ModFile{
		Sources: map[string]SourceConfig{},
	}
	sum := &SumFile{
		Modules: map[string]LockedModule{
			"foo": {
				Source:  "github:owner/repo/path",
				Version: "v1.2.3",
			},
		},
	}

	got, err := mod.ResolveModuleSource("foo", "github:owner/repo/path", "/tmp/modules", sum)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Provider != "github" {
		t.Fatalf("provider mismatch: got=%q", got.Provider)
	}
	if got.Ref != "owner/repo/path" {
		t.Fatalf("ref mismatch: got=%q", got.Ref)
	}
	if got.Version != "v1.2.3" {
		t.Fatalf("version mismatch: got=%q", got.Version)
	}
}

func TestResolveModuleSourceLockMismatch(t *testing.T) {
	t.Parallel()

	mod := &ModFile{
		Sources: map[string]SourceConfig{},
	}
	sum := &SumFile{
		Modules: map[string]LockedModule{
			"foo": {
				Source:  "github:owner/repo/other",
				Version: "v1.2.3",
			},
		},
	}

	_, err := mod.ResolveModuleSource("foo", "github:owner/repo/path", "/tmp/modules", sum)
	if err == nil {
		t.Fatal("expected lock mismatch error")
	}
}

func TestResolveModuleSourceLocalAlias(t *testing.T) {
	t.Parallel()

	mod := &ModFile{
		Sources: map[string]SourceConfig{
			"repo": {
				Provider: "local",
				Path:     "shared-modules",
			},
		},
	}

	got, err := mod.ResolveModuleSource("foo", "repo:base16-vim", "/home/user/dotfiles/modules", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := filepath.Clean("/home/user/dotfiles/shared-modules/base16-vim")
	if filepath.Clean(got.Ref) != want {
		t.Fatalf("ref mismatch: got=%q want=%q", got.Ref, want)
	}
}

func TestResolveModuleSourceCoreRejectsVersion(t *testing.T) {
	t.Parallel()

	mod := &ModFile{Sources: map[string]SourceConfig{}}
	_, err := mod.ResolveModuleSource("icons", "core:base16-icons-linux@v1", "/tmp/modules", nil)
	if err == nil {
		t.Fatal("expected version validation error")
	}
}

func TestResolveModuleSourceCoreDefaultWithoutModEntry(t *testing.T) {
	t.Parallel()

	mod := &ModFile{Sources: map[string]SourceConfig{}}

	got, err := mod.ResolveModuleSource("icons", "", "/tmp/modules", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Provider != "core" {
		t.Fatalf("provider mismatch: got=%q", got.Provider)
	}
	if got.Ref != "base16-icons-linux" {
		t.Fatalf("ref mismatch: got=%q", got.Ref)
	}
}
