package apps

import (
	"context"
	"fmt"
	"runtime"
	"strings"

	"workspaced/pkg/modfile"
	"workspaced/pkg/tool/backend"
	"workspaced/pkg/tool/backend/catalog"
	"workspaced/pkg/tool/backend/github"
	providerinstall "workspaced/pkg/tool/backend/install"
	"workspaced/pkg/tool/checks"
)

func init() {
	catalog.RegisterTool("terraform", newTerraform)
}

// terraformTool uses GitHub for version discovery but downloads artifacts from
// releases.hashicorp.com (GitHub release entries have no assets).
type terraformTool struct {
	inner backend.Tool
}

func newTerraform() (backend.Tool, error) {
	inner, err := github.NewTool("hashicorp/terraform", "terraform")
	if err != nil {
		return nil, err
	}
	return &terraformTool{inner: inner}, nil
}

func (t *terraformTool) ListVersions(ctx context.Context) ([]string, error) {
	vers, err := t.inner.ListVersions(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(vers))
	for _, v := range vers {
		v = strings.TrimPrefix(strings.TrimSpace(v), "v")
		if v == "" {
			continue
		}
		out = append(out, v)
	}
	return out, nil
}

func (t *terraformTool) Install(ctx context.Context, version string, destDir string) error {
	arts, err := t.ListArtifacts(ctx, version)
	if err != nil {
		return err
	}
	if len(arts) == 0 {
		return ErrNoPlatformArtifact
	}
	return t.InstallArtifact(ctx, arts[0], destDir)
}

func (t *terraformTool) EnrichLockfile(entry *modfile.RenovateDependency) {
	entry.DepName = "hashicorp/terraform"
	entry.Datasource = "github-releases"
	entry.Versioning = "semver"
}

func (t *terraformTool) ListArtifacts(ctx context.Context, version string) ([]backend.Artifact, error) {
	v := strings.TrimPrefix(strings.TrimSpace(version), "v")
	if v == "" || v == "latest" {
		vers, err := t.ListVersions(ctx)
		if err != nil {
			return nil, err
		}
		if len(vers) == 0 {
			return nil, ErrNoVersions
		}
		v = vers[0]
	}

	osName, arch, ok := terraformPlatform()
	if !ok {
		return nil, fmt.Errorf("%w: %s/%s", ErrNoPlatformArtifact, runtime.GOOS, runtime.GOARCH)
	}
	filename := fmt.Sprintf("terraform_%s_%s_%s.zip", v, osName, arch)
	url := fmt.Sprintf("https://releases.hashicorp.com/terraform/%s/%s", v, filename)
	return []backend.Artifact{{
		OS:   runtime.GOOS,
		Arch: runtime.GOARCH,
		URL:  url,
	}}, nil
}

func (t *terraformTool) InstallArtifact(ctx context.Context, artifact backend.Artifact, destDir string) error {
	return providerinstall.InstallArtifact(ctx, artifact, destDir, providerinstall.DownloadOptions{})
}

func (t *terraformTool) InstallChecks() []checks.Check {
	return checks.Checks(checks.Binary("terraform"))
}

func terraformPlatform() (osName, arch string, ok bool) {
	switch runtime.GOOS {
	case "linux", "darwin", "windows", "freebsd":
		osName = runtime.GOOS
	default:
		return "", "", false
	}
	switch runtime.GOARCH {
	case "amd64", "arm64", "386", "arm":
		arch = runtime.GOARCH
	default:
		return "", "", false
	}
	return osName, arch, true
}
