package modfile

import "context"

type SourceProvider interface {
	ID() string
	ResolvePath(ctx context.Context, alias string, src SourceConfig, rel string, modulesBaseDir string) (string, error)
	LockHash(ctx context.Context, alias string, src SourceConfig, modulesBaseDir string) (string, SourceConfig, error)
	Normalize(src SourceConfig) SourceConfig
}

var sourceProviders = map[string]SourceProvider{}

func RegisterSourceProvider(p SourceProvider) {
	sourceProviders[p.ID()] = p
}

func getSourceProvider(id string) (SourceProvider, bool) {
	p, ok := sourceProviders[id]
	return p, ok
}
