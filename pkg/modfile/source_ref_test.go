package modfile_test

import (
	"context"
	"path/filepath"
	"testing"
	"workspaced/pkg/modfile"
	_ "workspaced/pkg/modfile/sourceprovider/prelude"
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

	got, ok, err := mod.TryResolveSourceRefToPath(context.Background(), "papirus:Papirus", "/home/lucasew/.dotfiles/modules")
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
	got, ok, err := mod.TryResolveSourceRefToPath(context.Background(), input, "/home/lucasew/.dotfiles/modules")
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
