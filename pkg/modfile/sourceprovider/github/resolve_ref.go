package github

import (
	"context"
	"fmt"
	"regexp"
	"strings"
)

var shaRefRe = regexp.MustCompile(`^[a-fA-F0-9]{7,40}$`)

func (s Source) ResolvePinnedTarballURL(ctx context.Context) (string, error) {
	if u := strings.TrimSpace(s.Config.URL); u != "" {
		return u, nil
	}
	repo := s.Repo()
	if repo == "" {
		return "", fmt.Errorf("github source requires repo")
	}

	ref := strings.TrimSpace(s.Config.Ref)
	if ref == "" {
		defaultBranch, err := s.resolveDefaultBranch(ctx, repo)
		if err != nil {
			return "", err
		}
		ref = defaultBranch
	}
	if shaRefRe.MatchString(ref) {
		return fmt.Sprintf("https://codeload.github.com/%s/tar.gz/%s", repo, ref), nil
	}

	sha, err := s.resolveCommitSHA(ctx, repo, ref)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("https://codeload.github.com/%s/tar.gz/%s", repo, sha), nil
}

func (s Source) resolveDefaultBranch(ctx context.Context, repo string) (string, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s", repo)
	var payload struct {
		DefaultBranch string `json:"default_branch"`
	}
	if err := s.GetJSON(ctx, url, &payload); err != nil {
		return "", fmt.Errorf("repo metadata lookup failed: %w", err)
	}
	if strings.TrimSpace(payload.DefaultBranch) == "" {
		return "", fmt.Errorf("missing default_branch in github response")
	}
	return strings.TrimSpace(payload.DefaultBranch), nil
}

func (s Source) resolveCommitSHA(ctx context.Context, repo string, ref string) (string, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/commits/%s", repo, ref)
	var payload struct {
		SHA string `json:"sha"`
	}
	if err := s.GetJSON(ctx, url, &payload); err != nil {
		return "", fmt.Errorf("commit lookup failed: %w", err)
	}
	if strings.TrimSpace(payload.SHA) == "" {
		return "", fmt.Errorf("missing sha in github response")
	}
	return strings.TrimSpace(payload.SHA), nil
}
