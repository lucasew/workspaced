package configcue

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"cuelang.org/go/cue/cuecontext"
)

func TestResolveRuntimeInputs_valid(t *testing.T) {
	cueCtx := cuecontext.New()
	v := cueCtx.CompileString(`{
		inputs: {
			self: {from: "self"}
			localmod: {from: "local:./modules/foo"}
			gh: {from: "github:org/repo", version: "v1.0.0"}
		}
	}`)
	if err := v.Err(); err != nil {
		t.Fatalf("compile cue: %v", err)
	}

	repoCue := filepath.Join(t.TempDir(), "workspaced.cue")
	out, err := resolveRuntimeInputs(v, []string{repoCue}, []Layer{{Name: "repo", Path: repoCue}})
	if err != nil {
		t.Fatalf("resolveRuntimeInputs: %v", err)
	}
	if got := out["self"]["path"]; got == nil || got == "" {
		t.Fatalf("self path missing: %#v", out["self"])
	}
	if got, ok := out["localmod"]["path"].(string); !ok || !strings.HasSuffix(filepath.Clean(got), filepath.Join("modules", "foo")) {
		t.Fatalf("localmod path = %#v", out["localmod"])
	}
	got, ok := out["gh"]["path"].(string)
	if !ok || !filepath.IsAbs(got) || !strings.Contains(got, filepath.Join(".cache", "workspaced", "sources", "github")) {
		t.Fatalf("gh path = %#v, want absolute cache path under .cache/workspaced/sources/github", out["gh"])
	}
}

func TestResolveRuntimeInputs_decodeError(t *testing.T) {
	cueCtx := cuecontext.New()
	// Shape that JSON-decodes but cannot map into inputCfg (string instead of object).
	v := cueCtx.CompileString(`{inputs: {bad: "not-an-object"}}`)
	if err := v.Err(); err != nil {
		t.Fatalf("compile cue: %v", err)
	}
	_, err := resolveRuntimeInputs(v, nil, nil)
	if err == nil {
		t.Fatal("expected error for non-object input")
	}
	if !errors.Is(err, ErrDecodeInputs) {
		t.Fatalf("error = %v, want ErrDecodeInputs", err)
	}
}

func TestDecodeReadyMap_skipsIncompleteFileType(t *testing.T) {
	cueCtx := cuecontext.New()
	v := cueCtx.CompileString(`{
		inputs: {self: {from: "self"}}
		file: {
			"x.toml": {
				type: "json" | "toml" | "yaml"
				values: {a: 1}
			}
		}
	}`)
	if err := v.Err(); err != nil {
		t.Fatalf("compile cue: %v", err)
	}
	if _, err := v.MarshalJSON(); err == nil {
		t.Fatal("full marshal should fail on incomplete file type")
	}
	got, err := decodeReadyMap(v)
	if err != nil {
		t.Fatalf("decodeReadyMap: %v", err)
	}
	inputs, _ := got["inputs"].(map[string]any)
	self, _ := inputs["self"].(map[string]any)
	if from, _ := self["from"].(string); from != "self" {
		t.Fatalf("inputs.self.from = %#v", got["inputs"])
	}
	file, _ := got["file"].(map[string]any)
	entry, _ := file["x.toml"].(map[string]any)
	values, _ := entry["values"].(map[string]any)
	if values["a"] != float64(1) && values["a"] != 1 {
		t.Fatalf("file values = %#v, want a=1 without requiring type", got["file"])
	}
	if _, ok := entry["type"]; ok {
		t.Fatalf("incomplete type should be omitted, got %#v", entry["type"])
	}
}

func TestResolveRuntimeInputs_empty(t *testing.T) {
	cueCtx := cuecontext.New()
	v := cueCtx.CompileString(`{modules: {}}`)
	if err := v.Err(); err != nil {
		t.Fatalf("compile cue: %v", err)
	}
	out, err := resolveRuntimeInputs(v, nil, nil)
	if err != nil {
		t.Fatalf("resolveRuntimeInputs: %v", err)
	}
	if out != nil {
		t.Fatalf("out = %#v, want nil", out)
	}
}
