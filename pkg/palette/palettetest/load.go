// Package palettetest loads shared fixtures under pkg/palette/testdata.
package palettetest

import (
	"image"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// Path returns the absolute path to a file under pkg/palette/testdata.
func Path(t testing.TB, name string) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Join(filepath.Dir(file), "..", "testdata", name)
}

// LoadImage opens and decodes a palette testdata image.
func LoadImage(t testing.TB, name string) image.Image {
	t.Helper()
	f, err := os.Open(Path(t, name))
	if err != nil {
		t.Fatalf("open testdata %s: %v", name, err)
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	if err != nil {
		t.Fatalf("decode testdata %s: %v", name, err)
	}
	return img
}
