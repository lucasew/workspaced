package lsp

import (
	"testing"
	"time"
)

func TestResolveLanguageExtensionFirst(t *testing.T) {
	t.Parallel()
	cfg := Config{
		Extensions:  map[string]string{".go": "go"},
		LanguageIDs: map[string]string{"python": "python", "go": "go_from_id"},
	}
	normalizeConfig(&cfg)

	if got := cfg.ResolveLanguage("file:///x/y/z.go", "python"); got != "go" {
		t.Fatalf("extension should win: got %q", got)
	}
	if got := cfg.ResolveLanguage("file:///x/y/z.py", "python"); got != "python" {
		t.Fatalf("language id fallback: got %q", got)
	}
	if got := cfg.ResolveLanguage("file:///x/y/z.rs", "rust"); got != "" {
		t.Fatalf("unmapped: got %q", got)
	}
}

func TestBindingsForOrderAndServerID(t *testing.T) {
	t.Parallel()
	cfg := Config{
		Languages: map[string]map[string]Attachment{
			"go": {
				"99_refactree": {Capabilities: map[string]bool{"references": true}},
				"00_gopls":     {Capabilities: map[string]bool{"hover": true}},
			},
		},
		Servers: map[string]Server{
			"gopls":     {Cmd: []string{"gopls"}},
			"refactree": {Cmd: []string{"refactree"}},
		},
	}
	b := cfg.BindingsFor("go")
	if len(b) != 2 {
		t.Fatalf("len=%d", len(b))
	}
	if b[0].ServerID != "gopls" || b[0].OrderKey != "00_gopls" {
		t.Fatalf("first=%+v", b[0])
	}
	if b[1].ServerID != "refactree" {
		t.Fatalf("second=%+v", b[1])
	}
	if !b[0].HasCapability("hover") || b[0].HasCapability("references") {
		t.Fatalf("gopls caps")
	}
	// empty caps = all
	all := LanguageBinding{ServerID: "x"}
	if !all.HasCapability("anything") {
		t.Fatal("empty caps should allow all")
	}
}

func TestTimeoutDefault(t *testing.T) {
	t.Parallel()
	if (Config{}).Timeout() != defaultRequestTimeout {
		t.Fatal("default")
	}
	if (Config{RequestTimeout: "2s"}).Timeout() != 2*time.Second {
		t.Fatal("parsed")
	}
	if (Config{RequestTimeout: "nope"}).Timeout() != defaultRequestTimeout {
		t.Fatal("bad parse fallback")
	}
}

func TestCapabilityForMethod(t *testing.T) {
	t.Parallel()
	if CapabilityForMethod("textDocument/hover") != "hover" {
		t.Fatal()
	}
	if CapabilityForMethod("workspace/symbol") != "workspaceSymbol" {
		t.Fatal()
	}
}

func TestServerIDFromOrderKey(t *testing.T) {
	t.Parallel()
	if serverIDFromOrderKey("00_gopls") != "gopls" {
		t.Fatal()
	}
	if serverIDFromOrderKey("gopls") != "gopls" {
		t.Fatal()
	}
}
