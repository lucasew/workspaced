package module

import (
	"context"
	"os"
	"workspaced/internal/configcue"
)

type ResolveRequest struct {
	ModuleName     string
	Ref            string
	Version        string
	ModuleConfig   map[string]any
	ModulesBaseDir string
	Config         *configcue.Config
}

type ResolvedFile struct {
	RelPath    string
	TargetBase string
	Mode       os.FileMode
	Info       string
	AbsPath    string
	Symlink    bool
}

type Provider interface {
	ID() string
	Name() string
	Resolve(ctx context.Context, req ResolveRequest) ([]ResolvedFile, error)
}

// SourceRefResolver resolves a source reference of the form "alias:path" (or
// "provider:target") into an absolute filesystem path using the workspace's
// defined inputs. It returns the resolved path and true when resolution
// happened through a registered source provider.
type SourceRefResolver func(ctx context.Context, spec, modulesBaseDir string) (string, bool, error)

// CoreModule is one of the built-in modules available under the "core" provider.
// Example: the module with Ref() == "place" is used via from: "core:place".
type CoreModule interface {
	Ref() string
	Resolve(ctx context.Context, req ResolveRequest) ([]ResolvedFile, error)

	// Prepare lets the core module rewrite its moduleConfig before Resolve is
	// called. This is primarily used to turn source refs ("someinput:subdir")
	// into real on-disk paths. The provided resolver should be used for that.
	Prepare(ctx context.Context, moduleConfig map[string]any, resolver SourceRefResolver, modulesBaseDir string) error
}
