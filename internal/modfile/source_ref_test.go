package modfile_test

import (
	"path/filepath"
	"testing"

	"workspaced/internal/modfile"
	_ "workspaced/internal/modfile/sourceprovider/prelude"
)

func TestTryResolveSourceRefToPath(t *testing.T) {
	t.Parallel()

	mod := &modfile.ModFile{
		Sources: map[string]modfile.SourceConfig{
			"papirus": {
				Provider: "local",
				Path:     "/tmp/papirus-icon-theme-20250501",
			},
		},
	}

	got, ok, err := mod.TryResolveSourceRefToPath(t.Context(), "papirus:Papirus", "/home/lucasew/.dotfiles/modules")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatal("expected source ref to be resolved")
	}
	want := filepath.Clean("/tmp/papirus-icon-theme-20250501/Papirus")
	if filepath.Clean(got) != want {
		t.Fatalf("resolved path mismatch: got=%q want=%q", got, want)
	}
}

func TestTryResolveSourceRefToPathPlainPath(t *testing.T) {
	t.Parallel()

	mod := &modfile.ModFile{
		Sources: map[string]modfile.SourceConfig{},
	}

	input := "/tmp/papirus-icon-theme-20250501/Papirus"
	got, ok, err := mod.TryResolveSourceRefToPath(t.Context(), input, "/home/lucasew/.dotfiles/modules")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Fatal("did not expect plain path to be treated as source ref")
	}
	if got != input {
		t.Fatalf("expected input passthrough: got=%q want=%q", got, input)
	}
}

func TestTryResolveSourceRefToPathSelf(t *testing.T) {
	t.Parallel()

	// bare self input (from: "self") has Provider "self", empty Path.
	// "self:." and "self:subdir" must resolve relative to workspace root (Dir of modulesBaseDir).
	mod := &modfile.ModFile{
		Sources: map[string]modfile.SourceConfig{
			"skills_local_skills": {
				Provider: "self",
			},
		},
	}

	// modulesBaseDir points at <workspace>/modules
	modulesBase := "/home/user/dotfiles/modules"
	wsRoot := "/home/user/dotfiles"

	got, ok, err := mod.TryResolveSourceRefToPath(t.Context(), "skills_local_skills:.", modulesBase)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatal("expected self ref to resolve")
	}
	if got != wsRoot {
		t.Fatalf("self:. got=%q want=%q", got, wsRoot)
	}

	got, ok, err = mod.TryResolveSourceRefToPath(t.Context(), "skills_local_skills:codex/skills", modulesBase)
	if err != nil || !ok {
		t.Fatalf("unexpected: %v ok=%v", err, ok)
	}
	want := filepath.Join(wsRoot, "codex/skills")
	if got != want {
		t.Fatalf("self:subdir got=%q want=%q", got, want)
	}
}

func TestTryResolveSourceRefToPathDirectSelf(t *testing.T) {
	t.Parallel()

	// direct self: form (no entry in Sources) should also work
	mod := &modfile.ModFile{Sources: map[string]modfile.SourceConfig{}}

	modulesBase := "/workspace/modules"
	got, ok, err := mod.TryResolveSourceRefToPath(t.Context(), "self:.", modulesBase)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !ok || got != "/workspace" {
		t.Fatalf("direct self:. got=%q ok=%v", got, ok)
	}
}
