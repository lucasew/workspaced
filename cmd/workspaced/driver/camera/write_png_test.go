package camera

import (
	"context"
	"image"
	"image/color"
	"os"
	"path/filepath"
	"testing"
)

func TestWritePNGAtomic(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "shot.png")
	// Seed truncated prior
	if err := os.WriteFile(path, []byte("not-a-png"), 0o644); err != nil {
		t.Fatal(err)
	}
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	img.Set(0, 0, color.RGBA{R: 255, A: 255})
	if err := writePNGAtomic(context.Background(), path, img); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path + ".tmp"); !os.IsNotExist(err) {
		t.Fatalf("tmp left behind: %v", err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(raw) < 8 || string(raw[:8]) != "\x89PNG\r\n\x1a\n" {
		t.Fatalf("not a PNG header: %q", raw[:min(16, len(raw))])
	}
}
