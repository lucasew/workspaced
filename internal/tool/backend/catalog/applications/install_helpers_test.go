package apps

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	_ "github.com/lucasew/workspaced/pkg/driver/httpclient/native"
	"github.com/lucasew/workspaced/pkg/logging"
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

func TestSortVersionsDesc(t *testing.T) {
	t.Parallel()

	t.Run("empty yields ErrNoVersions", func(t *testing.T) {
		t.Parallel()
		_, err := sortVersionsDesc(nil)
		if !errors.Is(err, ErrNoVersions) {
			t.Fatalf("got %v, want ErrNoVersions", err)
		}
		_, err = sortVersionsDesc([]string{})
		if !errors.Is(err, ErrNoVersions) {
			t.Fatalf("got %v, want ErrNoVersions", err)
		}
	})

	t.Run("newest first", func(t *testing.T) {
		t.Parallel()
		got, err := sortVersionsDesc([]string{"1.2.0", "2.0.0", "1.10.0"})
		if err != nil {
			t.Fatal(err)
		}
		want := []string{"2.0.0", "1.10.0", "1.2.0"}
		if len(got) != len(want) {
			t.Fatalf("got %v, want %v", got, want)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("got %v, want %v", got, want)
			}
		}
	})
}

func TestHTTPGetHelpers(t *testing.T) {
	t.Parallel()
	ctx := logging.NewWriterContext(t.Output())

	t.Run("getBytes and configure", func(t *testing.T) {
		t.Parallel()
		var gotAccept string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotAccept = r.Header.Get("Accept")
			if _, err := w.Write([]byte("hello")); err != nil {
				t.Errorf("write: %v", err)
			}
		}))
		t.Cleanup(srv.Close)

		b, err := getBytes(ctx, srv.URL, func(req *http.Request) {
			req.Header.Set("Accept", "text/plain")
		})
		if err != nil {
			t.Fatal(err)
		}
		if string(b) != "hello" {
			t.Fatalf("got %q, want hello", b)
		}
		if gotAccept != "text/plain" {
			t.Fatalf("Accept = %q, want text/plain", gotAccept)
		}
	})

	t.Run("getJSON", func(t *testing.T) {
		t.Parallel()
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if _, err := w.Write([]byte(`{"version":"1.2.3"}`)); err != nil {
				t.Errorf("write: %v", err)
			}
		}))
		t.Cleanup(srv.Close)

		var dest struct {
			Version string `json:"version"`
		}
		if err := getJSON(ctx, srv.URL, &dest); err != nil {
			t.Fatal(err)
		}
		if dest.Version != "1.2.3" {
			t.Fatalf("got %q, want 1.2.3", dest.Version)
		}
	})

	t.Run("non-OK", func(t *testing.T) {
		t.Parallel()
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "nope", http.StatusNotFound)
		}))
		t.Cleanup(srv.Close)

		_, err := getBytes(ctx, srv.URL)
		var ue unexpectedHTTPStatusError
		if !errors.As(err, &ue) {
			t.Fatalf("got %v, want unexpectedHTTPStatusError", err)
		}
	})
}
