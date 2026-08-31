package github

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/lucasew/workspaced/internal/githubutil"
	"github.com/lucasew/workspaced/pkg/driver"
	httpclientdriver "github.com/lucasew/workspaced/pkg/driver/httpclient"
	"github.com/lucasew/workspaced/pkg/logging"
	"net/http"
)

func (s Source) GetJSON(ctx context.Context, url string, out any) error {
	httpDriver, err := driver.Get[httpclientdriver.Driver](ctx)
	if err != nil {
		return fmt.Errorf("get http client driver: %w", err)
	}

	req, err := githubutil.NewAPIRequest(ctx, http.MethodGet, url)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := httpDriver.Client().Do(req)
	if err != nil {
		return err
	}
	defer logging.Close(ctx, resp.Body)
	if resp.StatusCode != http.StatusOK {
		hint := ""
		if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusForbidden {
			if githubutil.Token(ctx) == "" {
				hint = " (private repos require GITHUB_TOKEN or 'gh auth login')"
			}
		}
		return fmt.Errorf("unexpected status: %s%s", resp.Status, hint)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}
