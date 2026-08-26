package filespine

import (
	"strings"
	"testing"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
)

func TestMountExternalPath(t *testing.T) {
	t.Parallel()
	src, err := Mount("app.dest")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(src, "app: {") || !strings.Contains(src, "dest?: _filespine.#Tree") {
		t.Fatalf("mount src:\n%s", src)
	}
	cueCtx := cuecontext.New()
	schema := cueCtx.CompileString(src, cue.Filename("filespine.cue"))
	if err := schema.Err(); err != nil {
		t.Fatal(err)
	}
	user := cueCtx.CompileString(`
app: dest: {
	"a.json": {type: "json", values: {port: 8080}}
	".bashrc": {type: "lines", values: {a: "umask 022"}}
}
`)
	if err := user.Err(); err != nil {
		t.Fatal(err)
	}
	v := schema.Unify(user)
	if err := v.Err(); err != nil {
		t.Fatal(err)
	}
	got, err := ParseAt(v, "app.dest")
	if err != nil {
		t.Fatal(err)
	}
	if got["a.json"].Type != TypeJSON {
		t.Fatalf("json type=%q", got["a.json"].Type)
	}
	if got[".bashrc"].Type != TypeLines {
		t.Fatalf("lines type=%q", got[".bashrc"].Type)
	}
}

func TestMountRejectsEnvSlot(t *testing.T) {
	t.Parallel()
	src, err := Mount("file")
	if err != nil {
		t.Fatal(err)
	}
	cueCtx := cuecontext.New()
	schema := cueCtx.CompileString(src)
	user := cueCtx.CompileString(`
file: "x": {
	type: "text"
	values: {a: {kind: "env", env: "EDITOR"}}
}
`)
	v := schema.Unify(user)
	if err := v.Err(); err == nil {
		t.Fatal("expected schema error")
	}
}

func TestMountPathErrors(t *testing.T) {
	t.Parallel()
	if _, err := Mount(""); err == nil {
		t.Fatal("empty path")
	}
	if _, err := Mount("foo..bar"); err == nil {
		t.Fatal("empty part")
	}
	if _, err := Mount("foo-bar.x"); err == nil {
		t.Fatal("invalid ident")
	}
}

func TestConstrain(t *testing.T) {
	t.Parallel()
	cueCtx := cuecontext.New()
	root := cueCtx.CompileString(`site: {}`)
	if err := root.Err(); err != nil {
		t.Fatal(err)
	}
	root, err := Constrain(root, "site.files")
	if err != nil {
		t.Fatal(err)
	}
	user := cueCtx.CompileString(`site: files: "n": {type: "text", values: {c: "hi"}}`)
	v := root.Unify(user)
	if err := v.Err(); err != nil {
		t.Fatal(err)
	}
	got, err := ParseAt(v, "site.files")
	if err != nil {
		t.Fatal(err)
	}
	if got["n"].Type != TypeText {
		t.Fatalf("type=%q", got["n"].Type)
	}
}
