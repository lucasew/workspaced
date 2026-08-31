package githubutil

import (
	"io"
	"net/http"
	"testing"

	"github.com/lucasew/workspaced/pkg/logging"
)

func TestNewAPIRequestHeaders(t *testing.T) {
	t.Setenv(githubTokenProbeEnv, githubTokenProbeVal)
	ctx := logging.NewWriterContext(io.Discard)
	req, err := NewAPIRequest(ctx, http.MethodGet, "https://api.github.com/repos/o/r/releases")
	if err != nil {
		t.Fatal(err)
	}
	if got := req.Header.Get("User-Agent"); got != UserAgent {
		t.Fatalf("User-Agent: got %q, want %q", got, UserAgent)
	}
	if got := req.Header.Get("X-GitHub-Api-Version"); got != APIVersion {
		t.Fatalf("X-GitHub-Api-Version: got %q, want %q", got, APIVersion)
	}
	if got := req.Header.Get("Authorization"); got != "" {
		t.Fatalf("Authorization with probe env: got %q, want empty", got)
	}
}

func TestApplyAPIHeadersNilSafe(t *testing.T) {
	ApplyAPIHeaders(logging.NewWriterContext(io.Discard), nil)
}
