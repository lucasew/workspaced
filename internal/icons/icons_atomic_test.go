package icons

import (
	"errors"
	"image"
	"image/color"
	"image/png"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

func TestWritePNGFileAtomic_Success(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "icon-cache.png")

	img := solidNRGBA(2, 2, color.NRGBA{R: 0xff, A: 0xff})
	if err := writePNGFileAtomic(path, img); err != nil {
		t.Fatalf("writePNGFileAtomic: %v", err)
	}

	got, err := decodePNG(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.Bounds().Dx() != 2 || got.Bounds().Dy() != 2 {
		t.Fatalf("decoded size = %v, want 2x2", got.Bounds())
	}
}

func TestWritePNGFileAtomic_FailureKeepsExisting(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "icon-cache.png")

	prior := solidNRGBA(1, 1, color.NRGBA{G: 0xff, A: 0xff})
	if err := writePNGFileAtomic(path, prior); err != nil {
		t.Fatal(err)
	}
	priorBytes, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	// Make dir non-writable so Create cannot open a new temp.
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chmod(dir, 0o755); err != nil {
			t.Errorf("chmod restore: %v", err)
		}
	})

	if err := writePNGFileAtomic(path, solidNRGBA(3, 3, color.NRGBA{B: 0xff, A: 0xff})); err == nil {
		t.Fatal("expected error when parent dir is not writable")
	}

	if err := os.Chmod(dir, 0o755); err != nil {
		t.Fatal(err)
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
	dir := t.TempDir()
	path := filepath.Join(dir, "icon-cache.png")

	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chmod(dir, 0o755); err != nil {
			t.Errorf("chmod restore: %v", err)
		}
	})

	if err := writePNGFileAtomic(path, solidNRGBA(2, 2, color.NRGBA{R: 1, A: 0xff})); err == nil {
		t.Fatal("expected error when parent dir is not writable")
	}
	if err := os.Chmod(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !errors.Is(err, fs.ErrNotExist) {
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
