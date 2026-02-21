package module

import (
	"context"
	"os"
	"workspaced/pkg/config"
)

type ResolveRequest struct {
	ModuleName     string
	Ref            string
	ModuleConfig   map[string]any
	ModulesBaseDir string
	Config         *config.GlobalConfig
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
