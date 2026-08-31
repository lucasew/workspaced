package githubutil

import (
	"context"
	"net/http"
)

// UserAgent identifies workspaced on GitHub HTTP calls (API and downloads).
const UserAgent = "workspaced (+https://github.com/lucasew/.dotfiles)"

// APIVersion is the GitHub REST API version header value.
const APIVersion = "2022-11-28"

// NewAPIRequest builds a GitHub REST request with User-Agent, API version, and auth.
func NewAPIRequest(ctx context.Context, method, rawURL string) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, method, rawURL, nil)
	if err != nil {
		return nil, err
	}
	ApplyAPIHeaders(ctx, req)
	return req, nil
}

// ApplyAPIHeaders sets User-Agent, X-GitHub-Api-Version, and Authorization when a token is available.
func ApplyAPIHeaders(ctx context.Context, req *http.Request) {
	if req == nil {
		return
	}
	req.Header.Set("User-Agent", UserAgent)
	req.Header.Set("X-GitHub-Api-Version", APIVersion)
	ApplyAuth(ctx, req)
}
