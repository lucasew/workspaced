package github

import (
	"strings"
	"workspaced/pkg/modfile"
)

type Source struct {
	Alias  string
	Config modfile.SourceConfig
}

func newSource(alias string, src modfile.SourceConfig) Source {
	cfg := src
	cfg.Provider = "github"
	cfg.Path = strings.TrimSpace(cfg.Path)
	cfg.Repo = normalizeGitHubRepo(cfg.Repo)
	cfg.Ref = strings.TrimSpace(cfg.Ref)
	cfg.URL = strings.TrimSpace(cfg.URL)
	return Source{
		Alias:  strings.TrimSpace(alias),
		Config: cfg,
	}
}

func (s Source) Repo() string {
	return strings.TrimSpace(s.Config.Repo)
}

func (s Source) Ref() string {
	ref := strings.TrimSpace(s.Config.Ref)
	if ref == "" {
		return "HEAD"
	}
	return ref
}

func (s Source) CacheKey() string {
	if u := strings.TrimSpace(s.Config.URL); u != "" {
		return "v4:url:" + u
	}
	return "v4:repo:" + s.Repo() + "@" + s.Ref()
}

func normalizeGitHubRepo(in string) string {
	repo := strings.Trim(strings.TrimSpace(in), "/")
	repo = strings.TrimPrefix(repo, "github:")
	repo = strings.Trim(repo, "/")
	return repo
}
