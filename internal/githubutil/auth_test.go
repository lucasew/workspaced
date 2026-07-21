package githubutil

import (
	"io"
	"net/http"
	"testing"

	"github.com/lucasew/workspaced/pkg/logging"
)

func TestResolveTokenSTOP(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", githubTokenStop)
	ctx := logging.NewWriterContext(io.Discard)
	got := resolveToken(ctx)
	if got != "" {
		t.Fatalf("resolveToken with GITHUB_TOKEN=STOP: got %q, want empty", got)
	}
}

func TestResolveTokenFromEnv(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "ghs_test_token")
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
