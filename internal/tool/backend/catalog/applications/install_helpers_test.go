package apps

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestResolveToolVersion(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	list := func(context.Context) ([]string, error) {
		return []string{"ruby-3.3.0", "ruby-3.2.0"}, nil
	}
	normalize := func(v string) string {
		v = strings.TrimSpace(v)
		v = strings.TrimPrefix(v, "ruby-")
		if v == "" || v == "latest" {
			return v
		}
		return v
	}

	t.Run("explicit version", func(t *testing.T) {
		t.Parallel()
		got, err := resolveToolVersion(ctx, "ruby-3.2.0", normalize, list)
		if err != nil {
			t.Fatal(err)
		}
		if got != "3.2.0" {
			t.Fatalf("got %q, want %q", got, "3.2.0")
		}
	})

	t.Run("latest normalizes listed version", func(t *testing.T) {
		t.Parallel()
		got, err := resolveToolVersion(ctx, "latest", normalize, list)
		if err != nil {
			t.Fatal(err)
		}
		if got != "3.3.0" {
			t.Fatalf("got %q, want %q", got, "3.3.0")
		}
	})

	t.Run("empty version treated as latest", func(t *testing.T) {
		t.Parallel()
		got, err := resolveToolVersion(ctx, "", normalize, list)
		if err != nil {
			t.Fatal(err)
		}
		if got != "3.3.0" {
			t.Fatalf("got %q, want %q", got, "3.3.0")
		}
	})

	t.Run("no versions", func(t *testing.T) {
		t.Parallel()
		_, err := resolveToolVersion(ctx, "latest", normalize, func(context.Context) ([]string, error) {
			return nil, nil
		})
		if !errors.Is(err, ErrNoVersions) {
			t.Fatalf("got %v, want ErrNoVersions", err)
		}
	})
}
