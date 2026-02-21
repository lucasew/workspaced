package github

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"workspaced/pkg/driver"
	httpclientdriver "workspaced/pkg/driver/httpclient"
)

func (s Source) GetJSON(ctx context.Context, url string, out any) error {
	httpDriver, err := driver.Get[httpclientdriver.Driver](ctx)
	if err != nil {
		return fmt.Errorf("failed to get http client driver: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "workspaced")
	resp, err := httpDriver.Client().Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status: %s", resp.Status)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}
