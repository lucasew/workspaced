package icons

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/lucasew/workspaced/pkg/logging"
)

func TestWritePNGFileAtomic_Success(t *testing.T) {
	ctx := logging.NewWriterContext(t.Output())
	dir := t.TempDir()
	path := filepath.Join(dir, "icon-cache.png")

	img := solidNRGBA(2, 2, color.NRGBA{R: 0xff, A: 0xff})
	if err := writePNGFileAtomic(ctx, path, img); err != nil {
		t.Fatalf("writePNGFileAtomic: %v", err)
	}

	got, err := decodePNG(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.Bounds().Dx() != 2 || got.Bounds().Dy() != 2 {
		t.Fatalf("decoded size = %v, want 2x2", got.Bounds())
	}
	if _, err := os.Stat(path + ".tmp"); !os.IsNotExist(err) {
		t.Fatalf("temp file still present, err=%v", err)
	}
}

func TestWritePNGFileAtomic_FailureKeepsExisting(t *testing.T) {
	ctx := logging.NewWriterContext(t.Output())
	dir := t.TempDir()
	path := filepath.Join(dir, "icon-cache.png")

	prior := solidNRGBA(1, 1, color.NRGBA{G: 0xff, A: 0xff})
	if err := writePNGFileAtomic(ctx, path, prior); err != nil {
		t.Fatal(err)
	}
	priorBytes, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	// Block temp create (path+".tmp" is a directory) so the write fails
	// before rename; an existing final path must stay untouched.
	if err := os.Mkdir(path+".tmp", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := writePNGFileAtomic(ctx, path, solidNRGBA(3, 3, color.NRGBA{B: 0xff, A: 0xff})); err == nil {
		t.Fatal("expected error when temp path is a directory")
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(priorBytes) {
		t.Fatalf("existing cache was mutated")
	}
}

func TestWritePNGFileAtomic_FailureLeavesNoFinal(t *testing.T) {
	ctx := logging.NewWriterContext(t.Output())
	dir := t.TempDir()
	path := filepath.Join(dir, "icon-cache.png")

	if err := os.Mkdir(path+".tmp", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := writePNGFileAtomic(ctx, path, solidNRGBA(2, 2, color.NRGBA{R: 1, A: 0xff})); err == nil {
		t.Fatal("expected error when temp path is a directory")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("final path should not exist after failed first write, err=%v", err)
	}
}

func solidNRGBA(w, h int, c color.NRGBA) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.SetNRGBA(x, y, c)
		}
	}
	return img
}

func decodePNG(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return png.Decode(f)
}
