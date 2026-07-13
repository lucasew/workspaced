package install

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"os"
	"path/filepath"
	"testing"
)

func TestStripTopLevelDir(t *testing.T) {
	root := t.TempDir()
	top := filepath.Join(root, "tool-v1.0.0")
	if err := os.MkdirAll(filepath.Join(top, "bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(top, "bin", "tool"), []byte("ok"), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := StripTopLevelDir(root); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(filepath.Join(root, "bin", "tool")); err != nil {
		t.Fatalf("expected stripped file: %v", err)
	}
	if _, err := os.Stat(top); !os.IsNotExist(err) {
		t.Fatalf("expected top-level directory to be removed, got %v", err)
	}
}

func TestExtractZipRejectsPathTraversal(t *testing.T) {
	root := t.TempDir()
	archive := filepath.Join(root, "bad.zip")
	outside := filepath.Join(root, "outside")
	dest := filepath.Join(root, "dest")

	file, err := os.Create(archive)
	if err != nil {
		t.Fatal(err)
	}
	writer := zip.NewWriter(file)
	entry, err := writer.Create("../outside")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := entry.Write([]byte("bad")); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}

	if err := Extract(t.Context(), archive, dest); err == nil {
		t.Fatal("expected path traversal error")
	}
	if _, err := os.Stat(outside); !os.IsNotExist(err) {
		t.Fatalf("path traversal wrote outside destination: %v", err)
	}
}

func TestExtractTarGzRejectsPathTraversal(t *testing.T) {
	root := t.TempDir()
	archive := filepath.Join(root, "bad.tar.gz")
	outside := filepath.Join(root, "outside")
	dest := filepath.Join(root, "dest")

	file, err := os.Create(archive)
	if err != nil {
		t.Fatal(err)
	}
	gzipWriter := gzip.NewWriter(file)
	tarWriter := tar.NewWriter(gzipWriter)
	if err := tarWriter.WriteHeader(&tar.Header{
		Name: "../outside",
		Mode: 0o644,
		Size: int64(len("bad")),
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := tarWriter.Write([]byte("bad")); err != nil {
		t.Fatal(err)
	}
	if err := tarWriter.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gzipWriter.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}

	if err := Extract(t.Context(), archive, dest); err == nil {
		t.Fatal("expected path traversal error")
	}
	if _, err := os.Stat(outside); !os.IsNotExist(err) {
		t.Fatalf("path traversal wrote outside destination: %v", err)
	}
}

func TestNormalizeInstalledBinaries(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "codex-x86_64-unknown-linux-musl")
	if err := os.WriteFile(src, []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := NormalizeInstalledBinaries(dir); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "codex")); err != nil {
		t.Fatalf("expected renamed codex binary: %v", err)
	}
	if _, err := os.Stat(src); !os.IsNotExist(err) {
		t.Fatal("expected old triple-named binary to be gone")
	}
}

func TestNormalizeBinaryName(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"sops-v3.13.2.linux.amd64", "sops"},
		{"sops-v3.13.2.darwin.arm64", "sops"},
		{"docker-compose-linux-x86_64", "docker-compose"},
		{"shfmt_linux_amd64", "shfmt"},
		{"shfmt_v3.13.1_linux_amd64", "shfmt"},
		{"shfmt_v3.13.1_linux_arm64", "shfmt"},
		{"codex-x86_64-unknown-linux-musl", "codex"},
		{"codex-aarch64-apple-darwin", "codex"},
		{"resvg", "resvg"},
		{"tool-v1.2.3", "tool"},
	}
	for _, tt := range tests {
		if got := NormalizeBinaryName(tt.in); got != tt.want {
			t.Errorf("NormalizeBinaryName(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
