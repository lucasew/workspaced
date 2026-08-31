package filespine

import (
	"bytes"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
)

func compileFileValue(t *testing.T, src string) cue.Value {
	t.Helper()
	v := cuecontext.New().CompileString(src)
	if err := v.Err(); err != nil {
		t.Fatalf("compile: %v", err)
	}
	file := v.LookupPath(cue.ParsePath("file"))
	if err := file.Err(); err != nil {
		t.Fatalf("file: %v", err)
	}
	return file
}

func TestParseSlotsAndEncodeLines(t *testing.T) {
	t.Parallel()
	fileVal := compileFileValue(t, `
file: ".bashrc": {
	type: "lines"
	values: {
		"10-path": "export PATH=foo"
		"20-alias": {kind: "text", text: "alias ll=ls"}
	}
}
`)
	got, err := Parse(fileVal)
	if err != nil {
		t.Fatal(err)
	}
	enc, err := Encode(got[".bashrc"])
	if err != nil {
		t.Fatal(err)
	}
	want := "export PATH=foo\nalias ll=ls"
	if string(enc) != want {
		t.Fatalf("encode = %q, want %q", enc, want)
	}
}

func TestParseBareStringAndStructRef(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	snippet := filepath.Join(dir, "extra.sh")
	if err := os.WriteFile(snippet, []byte("echo hi"), 0o644); err != nil {
		t.Fatal(err)
	}
	fileVal := compileFileValue(t, `
file: ".profile": {
	type: "lines"
	values: {
		head: "umask 022"
		tail: {kind: "ref", ref: "`+snippet+`"}
	}
}
`)
	got, err := Parse(fileVal)
	if err != nil {
		t.Fatal(err)
	}
	enc, err := Encode(got[".profile"])
	if err != nil {
		t.Fatal(err)
	}
	want := "umask 022\necho hi"
	if string(enc) != want {
		t.Fatalf("encode = %q, want %q", enc, want)
	}
}

func TestTextRequiresOneKey(t *testing.T) {
	t.Parallel()
	fileVal := compileFileValue(t, `
file: "a.txt": {
	type: "text"
	values: {x: "one", y: "two"}
}
`)
	_, err := Parse(fileVal)
	if !errors.Is(err, ErrTextKeyCount) {
		t.Fatalf("err = %v, want ErrTextKeyCount", err)
	}
}

func TestUnknownSlotKind(t *testing.T) {
	t.Parallel()
	fileVal := compileFileValue(t, `
file: "a.txt": {
	type: "text"
	values: {x: {kind: "env", env: "EDITOR"}}
}
`)
	_, err := Parse(fileVal)
	if !errors.Is(err, ErrUnknownSlotKind) {
		t.Fatalf("err = %v, want ErrUnknownSlotKind", err)
	}
}

func TestMergeTypeConflict(t *testing.T) {
	t.Parallel()
	declared := map[string]File{
		".bashrc": {Path: ".bashrc", Type: TypeLines, Values: map[string]Slot{
			"a": {Kind: KindText, Text: "a"},
		}},
	}
	_, err := Merge(declared, []Contribution{{
		Path: ".bashrc",
		Type: TypeText,
		Key:  "content",
		Slot: Slot{Kind: KindText, Text: "nope"},
	}})
	if !errors.Is(err, ErrTypeConflict) {
		t.Fatalf("err = %v, want ErrTypeConflict", err)
	}
}

func TestMergeLinesKeys(t *testing.T) {
	t.Parallel()
	declared := map[string]File{
		".bashrc": {Path: ".bashrc", Type: TypeLines, Values: map[string]Slot{
			"10-a": {Kind: KindText, Text: "A"},
		}},
	}
	got, err := Merge(declared, []Contribution{{
		Path: ".bashrc",
		Type: TypeLines,
		Key:  "20-b",
		Slot: Slot{Kind: KindText, Text: "B"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	enc, err := Encode(got[".bashrc"])
	if err != nil {
		t.Fatal(err)
	}
	if string(enc) != "A\nB" {
		t.Fatalf("encode = %q", enc)
	}
}

func TestFSOpenCombined(t *testing.T) {
	t.Parallel()
	dest := NewFS(map[string]File{
		".bashrc": {
			Path: ".bashrc",
			Type: TypeLines,
			Values: map[string]Slot{
				"10": {Kind: KindText, Text: "one"},
				"20": {Kind: KindText, Text: "two"},
			},
			Mode: 0o644,
		},
		".config/git/config": {
			Path: ".config/git/config",
			Type: TypeText,
			Values: map[string]Slot{
				"content": {Kind: KindText, Text: "[user]\n"},
			},
			Mode: 0o644,
		},
	})
	got, err := fs.ReadFile(dest, ".bashrc")
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "one\ntwo" {
		t.Fatalf("bashrc = %q", got)
	}
	if err := fs.WalkDir(dest, ".", func(name string, d fs.DirEntry, err error) error {
		return err
	}); err != nil {
		t.Fatal(err)
	}
	names := []string{}
	if err := fs.WalkDir(dest, ".", func(name string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			names = append(names, name)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	want := []string{".bashrc", ".config/git/config"}
	if len(names) != len(want) {
		t.Fatalf("walk = %v, want %v", names, want)
	}
	for i := range want {
		if names[i] != want[i] {
			t.Fatalf("walk = %v, want %v", names, want)
		}
	}
}

func TestInvalidPathRejected(t *testing.T) {
	t.Parallel()
	fileVal := compileFileValue(t, `
file: "../etc/passwd": {
	type: "text"
	values: {content: "x"}
}
`)
	_, err := Parse(fileVal)
	if !errors.Is(err, ErrInvalidPath) {
		t.Fatalf("err = %v, want ErrInvalidPath", err)
	}
}

func TestOpenDoesNotExposeKeys(t *testing.T) {
	t.Parallel()
	dest := NewFS(map[string]File{
		".bashrc": {
			Path: ".bashrc",
			Type: TypeLines,
			Values: map[string]Slot{
				"10-path": {Kind: KindText, Text: "export PATH=1"},
			},
		},
	})
	_, err := dest.Open(".bashrc/10-path")
	if !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("Open key = %v, want ErrNotExist", err)
	}
	f, err := dest.Open(".bashrc")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	data, err := io.ReadAll(f)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "export PATH=1" {
		t.Fatalf("got %q", data)
	}
}

func TestEncodeStructuredFormats(t *testing.T) {
	t.Parallel()
	fileVal := compileFileValue(t, `
file: {
	"a.json": {type: "json", values: {name: "x", port: 8080, on: true}}
	"a.toml": {type: "toml", values: {name: "x", port: 8080}}
	"a.yaml": {type: "yaml", values: {name: "x", nested: {a: true}}}
	"a.ini":  {type: "ini", values: {editor: "vim", core: {bare: true, filemode: true}}}
	"a.xml":  {type: "xml", values: {cfg: {name: "x", nested: {a: true}, port: 8080}}}
}
`)
	got, err := Parse(fileVal)
	if err != nil {
		t.Fatal(err)
	}
	jsonOut, err := Encode(got["a.json"])
	if err != nil {
		t.Fatal(err)
	}
	wantJSON := "{\n  \"name\": \"x\",\n  \"on\": true,\n  \"port\": 8080\n}\n"
	if string(jsonOut) != wantJSON {
		t.Fatalf("json = %q, want %q", jsonOut, wantJSON)
	}

	tomlOut, err := Encode(got["a.toml"])
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(tomlOut, []byte("name")) || !bytes.Contains(tomlOut, []byte("8080")) {
		t.Fatalf("toml = %q", tomlOut)
	}

	yamlOut, err := Encode(got["a.yaml"])
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(yamlOut, []byte("name: x")) || !bytes.Contains(yamlOut, []byte("a: true")) {
		t.Fatalf("yaml = %q", yamlOut)
	}

	iniOut, err := Encode(got["a.ini"])
	if err != nil {
		t.Fatal(err)
	}
	wantINI := "editor = vim\n[core]\nbare = true\nfilemode = true\n"
	if string(iniOut) != wantINI {
		t.Fatalf("ini = %q, want %q", iniOut, wantINI)
	}

	xmlOut, err := Encode(got["a.xml"])
	if err != nil {
		t.Fatal(err)
	}
	wantXML := "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<cfg>\n  <name>x</name>\n  <nested>\n    <a>true</a>\n  </nested>\n  <port>8080</port>\n</cfg>\n"
	if string(xmlOut) != wantXML {
		t.Fatalf("xml = %q, want %q", xmlOut, wantXML)
	}
}

func TestXMLRepeatsListItems(t *testing.T) {
	t.Parallel()
	f := File{
		Path: "a.xml",
		Type: TypeXML,
		Data: map[string]any{
			"items": map[string]any{
				"item": []any{"a", "b"},
			},
		},
	}
	out, err := Encode(f)
	if err != nil {
		t.Fatal(err)
	}
	want := "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<items>\n  <item>a</item>\n  <item>b</item>\n</items>\n"
	if string(out) != want {
		t.Fatalf("xml = %q, want %q", out, want)
	}
}

func TestXMLEscapesText(t *testing.T) {
	t.Parallel()
	f := File{
		Path: "a.xml",
		Type: TypeXML,
		Data: map[string]any{"note": "a<b>&c"},
	}
	out, err := Encode(f)
	if err != nil {
		t.Fatal(err)
	}
	want := "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<note>a&lt;b&gt;&amp;c</note>\n"
	if string(out) != want {
		t.Fatalf("xml = %q, want %q", out, want)
	}
}

func TestXMLRejectsMultipleRoots(t *testing.T) {
	t.Parallel()
	f := File{
		Path: "a.xml",
		Type: TypeXML,
		Data: map[string]any{"a": "1", "b": "2"},
	}
	_, err := Encode(f)
	if !errors.Is(err, ErrXMLRoot) {
		t.Fatalf("err = %v, want ErrXMLRoot", err)
	}
}

func TestXMLRejectsBadName(t *testing.T) {
	t.Parallel()
	f := File{
		Path: "a.xml",
		Type: TypeXML,
		Data: map[string]any{"1cfg": "x"},
	}
	_, err := Encode(f)
	if !errors.Is(err, ErrXMLName) {
		t.Fatalf("err = %v, want ErrXMLName", err)
	}
}

func TestStructuredRejectsUnsupportedScalar(t *testing.T) {
	t.Parallel()
	for _, typ := range []string{TypeINI, TypeXML} {
		t.Run(typ, func(t *testing.T) {
			t.Parallel()
			f := File{
				Path: "a." + typ,
				Type: typ,
				Data: map[string]any{"x": []byte("nope")},
			}
			_, err := Encode(f)
			if !errors.Is(err, ErrUnsupportedValue) {
				t.Fatalf("err = %v, want ErrUnsupportedValue", err)
			}
		})
	}
}

func TestINIRejectsNestedSection(t *testing.T) {
	t.Parallel()
	f := File{
		Path: "a.ini",
		Type: TypeINI,
		Data: map[string]any{
			"core": map[string]any{
				"deep": map[string]any{"x": "y"},
			},
		},
	}
	_, err := Encode(f)
	if !errors.Is(err, ErrINISection) {
		t.Fatalf("err = %v, want ErrINISection", err)
	}
}

func TestStructuredOpen(t *testing.T) {
	t.Parallel()
	fileVal := compileFileValue(t, `
file: "cfg.json": {type: "json", values: {ok: true}}
`)
	parsed, err := Parse(fileVal)
	if err != nil {
		t.Fatal(err)
	}
	got, err := fs.ReadFile(NewFS(parsed), "cfg.json")
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "{\n  \"ok\": true\n}\n" {
		t.Fatalf("got %q", got)
	}
}
