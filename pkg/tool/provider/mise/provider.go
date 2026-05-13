package mise

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"workspaced/pkg/miseutil"
	"workspaced/pkg/tool"
	"workspaced/pkg/tool/provider"
)

type Provider struct{}

func init() {
	tool.RegisterProvider(&Provider{})
}

func (p *Provider) ID() string   { return "mise" }
func (p *Provider) Name() string { return "mise" }

func (p *Provider) ParsePackage(spec string) (provider.PackageConfig, error) {
	spec = strings.TrimSpace(spec)
	if spec == "" {
		return provider.PackageConfig{}, fmt.Errorf("mise spec cannot be empty")
	}
	return provider.PackageConfig{
		Provider: "mise",
		Spec:     spec,
	}, nil
}

func (p *Provider) ListVersions(ctx context.Context, pkg provider.PackageConfig) ([]string, error) {
	version, err := miseutil.Latest(ctx, pkg.Spec)
	if err != nil {
		return nil, err
	}
	if version == "" {
		return nil, fmt.Errorf("mise latest returned empty version for %q", pkg.Spec)
	}
	return []string{version}, nil
}

func (p *Provider) GetArtifacts(ctx context.Context, pkg provider.PackageConfig, version string) ([]provider.Artifact, error) {
	_ = ctx
	return []provider.Artifact{{
		URL: pkg.Spec + "@" + strings.TrimSpace(version),
	}}, nil
}

func (p *Provider) Install(ctx context.Context, artifact provider.Artifact, destPath string) error {
	_ = destPath
	spec := strings.TrimSpace(artifact.URL)
	if spec == "" {
		return fmt.Errorf("missing mise artifact spec")
	}
	return miseutil.Run(ctx, "install", spec)
}

func (p *Provider) EnsureBinary(ctx context.Context, pkg provider.PackageConfig, version string, cmdName string, destPath string) (string, error) {
	toolSpec := strings.TrimSpace(pkg.Spec) + "@" + strings.TrimSpace(version)

	binPath, err := miseutil.ResolveBinPath(ctx, cmdName, toolSpec)
	if err == nil {
		return ensureSymlink(destPath, binPath, cmdName)
	}

	if err := miseutil.Run(ctx, "install", toolSpec); err != nil {
		return "", err
	}

	binPath, err = miseutil.ResolveBinPath(ctx, cmdName, toolSpec)
	if err != nil {
		return "", err
	}
	return ensureSymlink(destPath, binPath, cmdName)
}

func ensureSymlink(destPath, binPath, cmdName string) (string, error) {
	if err := os.MkdirAll(destPath, 0755); err != nil {
		return "", err
	}

	linkPath := filepath.Join(destPath, "bin", cmdName)
	if err := os.MkdirAll(filepath.Dir(linkPath), 0755); err != nil {
		return "", err
	}

	if _, err := os.Lstat(linkPath); err == nil {
		if err := os.Remove(linkPath); err != nil {
			return "", err
		}
	}

	if err := os.Symlink(binPath, linkPath); err != nil {
		return "", err
	}

	return linkPath, nil
}
