package apps

import (
	"os"
	"path/filepath"
	"testing"

	"workspaced/internal/tool/backend"
)

func TestFixRubyShebangs(t *testing.T) {
	dir := t.TempDir()
	binDir := filepath.Join(dir, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Create a fake ruby binary (not a script)
	rubyBin := filepath.Join(binDir, "ruby")
	if err := os.WriteFile(rubyBin, []byte("ELF..."), 0o755); err != nil {
		t.Fatal(err)
	}

	// Create a script with the bad hostedtoolcache shebang (exact case from report)
	badScript := filepath.Join(binDir, "bundle")
	badContent := "#!/opt/hostedtoolcache/Ruby/4.0.5/x64/bin/ruby\n" +
		"puts 'hello'\n"
	if err := os.WriteFile(badScript, []byte(badContent), 0o755); err != nil {
		t.Fatal(err)
	}

	// Another with args
	badWithArgs := filepath.Join(binDir, "rake")
	badWithArgsContent := "#!/opt/hostedtoolcache/Ruby/4.0.5/x64/bin/ruby -w\n" +
		"puts 'rake'\n"
	if err := os.WriteFile(badWithArgs, []byte(badWithArgsContent), 0o755); err != nil {
		t.Fatal(err)
	}

	// A good one already (should stay)
	good := filepath.Join(binDir, "good")
	goodRuby := filepath.Join(binDir, "ruby")
	goodContent := "#!" + goodRuby + "\nputs 'ok'\n"
	if err := os.WriteFile(good, []byte(goodContent), 0o755); err != nil {
		t.Fatal(err)
	}

	// A non-ruby shebang (should be untouched)
	other := filepath.Join(binDir, "other")
	if err := os.WriteFile(other, []byte("#!/bin/sh\necho hi\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	tool := &rubyTool{}
	if err := tool.fixRubyShebangs(dir); err != nil {
		t.Fatalf("fixRubyShebangs: %v", err)
	}

	// Check badScript
	got, _ := os.ReadFile(badScript)
	want := "#!" + goodRuby + "\nputs 'hello'\n"
	if string(got) != want {
		t.Errorf("bundle shebang: got %q want %q", string(got), want)
	}

	// Check with args preserved
	got, _ = os.ReadFile(badWithArgs)
	want = "#!" + goodRuby + " -w\nputs 'rake'\n"
	if string(got) != want {
		t.Errorf("rake shebang: got %q want %q", string(got), want)
	}

	// good unchanged
	got, _ = os.ReadFile(good)
	if string(got) != goodContent {
		t.Errorf("good changed: %q", string(got))
	}

	// other unchanged
	got, _ = os.ReadFile(other)
	if !bytesHasPrefix(got, []byte("#!/bin/sh")) {
		t.Errorf("other shebang changed unexpectedly")
	}
}

func bytesHasPrefix(b, prefix []byte) bool {
	return len(b) >= len(prefix) && string(b[:len(prefix)]) == string(prefix)
}

func TestRubyToolImplementsInstallFixer(t *testing.T) {
	tool := &rubyTool{}
	var _ backend.InstallFixer = tool // compile-time check

	// Also via the catalog registration path
	// (avoiding direct import of internal/tool to prevent cycles in some builds)
	// We just exercise the method we already have.
	dir := t.TempDir()
	bin := filepath.Join(dir, "bin")
	_ = os.MkdirAll(bin, 0o755)
	_ = os.WriteFile(filepath.Join(bin, "ruby"), []byte("fake"), 0o755)
	script := filepath.Join(bin, "irb")
	_ = os.WriteFile(script, []byte("#!/opt/hostedtoolcache/Ruby/4.0.5/x64/bin/ruby\n# gem wrapper\n"), 0o755)

	if err := tool.Fix(t.Context(), dir); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(script)
	if !bytesHasPrefix(b, []byte("#!"+filepath.Join(dir, "bin", "ruby"))) {
		t.Errorf("Fix via interface did not rewrite: %q", string(b[:60]))
	}
}
