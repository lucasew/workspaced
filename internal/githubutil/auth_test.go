package githubutil

import (
	"context"
	"errors"
	"io"
	"net/http"
	"path/filepath"
	"testing"

	"github.com/lucasew/workspaced/pkg/logging"
)

func TestResolveTokenSTOP(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", githubTokenStop)
	t.Setenv(githubTokenProbeEnv, "")
	ctx := logging.NewWriterContext(io.Discard)
	got := resolveToken(ctx)
	if got != "" {
		t.Fatalf("resolveToken with GITHUB_TOKEN=STOP: got %q, want empty", got)
	}
}

func TestResolveTokenFromEnv(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "ghs_test_token")
	t.Setenv(githubTokenProbeEnv, "")
	ctx := logging.NewWriterContext(io.Discard)
	got := resolveToken(ctx)
	if got != "ghs_test_token" {
		t.Fatalf("resolveToken: got %q, want ghs_test_token", got)
	}
}

func TestResolveTokenSTOPNotUsedAsBearer(t *testing.T) {
	// ApplyAuth goes through Token (sync.Once). Exercise resolveToken + header
	// policy without relying on process-global Token cache.
	t.Setenv("GITHUB_TOKEN", githubTokenStop)
	t.Setenv(githubTokenProbeEnv, "")
	ctx := logging.NewWriterContext(io.Discard)
	if tok := resolveToken(ctx); tok != "" {
		t.Fatalf("token: got %q, want empty", tok)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://example.com", nil)
	if err != nil {
		t.Fatal(err)
	}
	// Mirror ApplyAuth's rule: only set Authorization when token is non-empty.
	if tok := resolveToken(ctx); tok != "" {
		req.Header.Set("Authorization", "Bearer "+tok)
	}
	if got := req.Header.Get("Authorization"); got != "" {
		t.Fatalf("Authorization header: got %q, want empty (STOP must not be sent as Bearer)", got)
	}
}

func TestTokenProbeEnvSkipsResolution(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv(githubTokenProbeEnv, githubTokenProbeVal)
	ctx := logging.NewWriterContext(io.Discard)
	// Token short-circuits on probe env before Once/env/gh.
	if got := Token(ctx); got != "" {
		t.Fatalf("Token with probe env: got %q, want empty", got)
	}
}

func TestResolveGHBinaryUsesLocatorWhenPATHMissing(t *testing.T) {
	t.Setenv("PATH", t.TempDir()) // empty PATH: no gh
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv(githubTokenProbeEnv, "")

	want := filepath.Join(t.TempDir(), "gh")
	SetGHLocator(func(ctx context.Context) (string, error) {
		return want, nil
	})
	t.Cleanup(func() { SetGHLocator(nil) })

	ctx := logging.NewWriterContext(io.Discard)
	got, err := resolveGHBinary(ctx)
	if err != nil {
		t.Fatalf("resolveGHBinary: %v", err)
	}
	if got != want {
		t.Fatalf("resolveGHBinary: got %q, want %q", got, want)
	}
}

func TestResolveGHBinaryLocatorError(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv(githubTokenProbeEnv, "")

	boom := errors.New("ensure failed")
	SetGHLocator(func(ctx context.Context) (string, error) {
		return "", boom
	})
	t.Cleanup(func() { SetGHLocator(nil) })

	ctx := logging.NewWriterContext(io.Discard)
	_, err := resolveGHBinary(ctx)
	if !errors.Is(err, boom) {
		t.Fatalf("resolveGHBinary: got %v, want %v", err, boom)
	}
}

func TestResolveGHBinaryNoLocator(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv(githubTokenProbeEnv, "")
	SetGHLocator(nil)

	ctx := logging.NewWriterContext(io.Discard)
	_, err := resolveGHBinary(ctx)
	if !errors.Is(err, errGHNotFound) {
		t.Fatalf("resolveGHBinary: got %v, want %v", err, errGHNotFound)
	}
}
