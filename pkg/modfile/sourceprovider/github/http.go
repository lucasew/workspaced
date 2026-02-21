package github

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

var githubHTTPClient = &http.Client{
	Timeout: 30 * time.Second,
}

func (s Source) GetJSON(ctx context.Context, url string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "workspaced")
	resp, err := githubHTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status: %s", resp.Status)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}
