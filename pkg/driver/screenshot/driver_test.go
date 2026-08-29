package screenshot

import (
	"strings"
	"testing"
)

func TestParseRectParts(t *testing.T) {
	t.Parallel()

	t.Run("valid", func(t *testing.T) {
		t.Parallel()
		r, err := ParseRectParts([]string{"10", "20", "300", "400"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if r.X != 10 || r.Y != 20 || r.Width != 300 || r.Height != 400 {
			t.Fatalf("got %+v", r)
		}
	})

	t.Run("wrong count", func(t *testing.T) {
		t.Parallel()
		if _, err := ParseRectParts([]string{"1", "2", "3"}); err == nil {
			t.Fatal("expected error for wrong field count")
		}
	})

	t.Run("non-integer", func(t *testing.T) {
		t.Parallel()
		_, err := ParseRectParts([]string{"10", "20", "abc", "400"})
		if err == nil {
			t.Fatal("expected error for non-integer field")
		}
		if !strings.Contains(err.Error(), "abc") {
			t.Fatalf("error should mention bad field: %v", err)
		}
	})
}
