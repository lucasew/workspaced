package configcue

import (
	"testing"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
	"github.com/lucasew/workspaced/pkg/filespine"
)

func TestFileSpineSchema(t *testing.T) {
	t.Parallel()
	schemaBytes, err := schemaFS.ReadFile("schema.cue")
	if err != nil {
		t.Fatal(err)
	}
	cueCtx := cuecontext.New()
	schema := cueCtx.CompileBytes(schemaBytes, cue.Filename("schema.cue"))
	if err := schema.Err(); err != nil {
		t.Fatalf("schema: %v", err)
	}
	schema, err = filespine.Constrain(schema, "workspaced.file")
	if err != nil {
		t.Fatalf("mount: %v", err)
	}

	unify := func(t *testing.T, user string) error {
		t.Helper()
		u := cueCtx.CompileString(user, cue.Filename("user.cue"))
		if err := u.Err(); err != nil {
			return err
		}
		v := schema.Unify(u)
		if err := v.Err(); err != nil {
			return err
		}
		file := v.LookupPath(cue.ParsePath("workspaced.file"))
		if err := file.Err(); err != nil {
			return err
		}
		_, err := file.MarshalJSON()
		return err
	}

	t.Run("accepts structured types", func(t *testing.T) {
		t.Parallel()
		err := unify(t, `
package workspaced
workspaced: file: {
	"a.json": {type: "json", values: {port: 8080, name: "x"}}
	"a.toml": {type: "toml", values: {port: 8080}}
	"a.yaml": {type: "yaml", values: {nested: {a: true}}}
	"a.ini":  {type: "ini", values: {core: {bare: true}}}
}
`)
		if err != nil {
			t.Fatalf("unify: %v", err)
		}
	})

	t.Run("accepts lines text and ref", func(t *testing.T) {
		t.Parallel()
		err := unify(t, `
package workspaced
workspaced: file: {
	".bashrc": {
		type: "lines"
		values: {
			a: "export PATH=1"
			b: {kind: "text", text: "alias ll=ls"}
		}
	}
	"readme": {
		type: "text"
		values: {content: "hi"}
	}
	"blob": {
		type: "ref"
		values: {src: {kind: "ref", ref: "/tmp/x"}}
	}
}
`)
		if err != nil {
			t.Fatalf("unify: %v", err)
		}
	})

	t.Run("rejects env slot", func(t *testing.T) {
		t.Parallel()
		err := unify(t, `
package workspaced
workspaced: file: "x": {
	type: "text"
	values: {a: {kind: "env", env: "EDITOR"}}
}
`)
		if err == nil {
			t.Fatal("expected schema error")
		}
	})

	t.Run("rejects unknown file type", func(t *testing.T) {
		t.Parallel()
		err := unify(t, `
package workspaced
workspaced: file: "x": {
	type: "xml"
	values: {a: "{}"}
}
`)
		if err == nil {
			t.Fatal("expected schema error")
		}
	})
}
