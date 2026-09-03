package screenshot

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/lucasew/workspaced/pkg/logging"
)

func TestCaptureViaCmd(t *testing.T) {
	t.Parallel()
	ctx := logging.NewWriterContext(t.Output())

	img := image.NewRGBA(image.Rect(0, 0, 2, 1))
	img.Set(0, 0, color.RGBA{R: 255, A: 255})
	img.Set(1, 0, color.RGBA{B: 255, A: 255})
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "shot.png")
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := CaptureViaCmd(ctx, "cat", path)
	if err != nil {
		t.Fatalf("CaptureViaCmd: %v", err)
	}
	if got.Bounds() != img.Bounds() {
		t.Fatalf("bounds=%v want %v", got.Bounds(), img.Bounds())
	}
	if c := color.RGBAModel.Convert(got.At(0, 0)).(color.RGBA); c.R != 255 || c.A != 255 {
		t.Fatalf("pixel(0,0)=%v want red", c)
	}
}

func TestCaptureViaCmdFailed(t *testing.T) {
	t.Parallel()
	ctx := logging.NewWriterContext(t.Output())
	_, err := CaptureViaCmd(ctx, "false")
	if err == nil {
		t.Fatal("expected command failure")
	}
	var ee *exec.ExitError
	if !errors.As(err, &ee) {
		t.Fatalf("err=%v want ExitError", err)
	}
}

func TestCaptureViaCmdBadImage(t *testing.T) {
	t.Parallel()
	ctx := logging.NewWriterContext(t.Output())
	path := filepath.Join(t.TempDir(), "not.png")
	if err := os.WriteFile(path, []byte("not-an-image"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := CaptureViaCmd(ctx, "cat", path)
	if err == nil {
		t.Fatal("expected decode failure")
	}
	if !errors.Is(err, image.ErrFormat) {
		t.Fatalf("err=%v want image.ErrFormat", err)
	}
}
