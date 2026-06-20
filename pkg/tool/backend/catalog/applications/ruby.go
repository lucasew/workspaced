package apps

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"workspaced/pkg/logging"
	"workspaced/pkg/modfile"
	"workspaced/pkg/semver"
	"workspaced/pkg/tool/backend"
	"workspaced/pkg/tool/backend/catalog"
	"workspaced/pkg/tool/backend/github"
	providerinstall "workspaced/pkg/tool/backend/install"
)

func init() {
	catalog.RegisterTool("ruby", newRuby)
}

type rubyTool struct {
	inner backend.Tool
}

func newRuby() (backend.Tool, error) {
	inner, err := github.NewTool("ruby/ruby-builder")
	if err != nil {
		return nil, err
	}
	return &rubyTool{inner: inner}, nil
}

func (t *rubyTool) ListVersions(ctx context.Context) ([]string, error) {
	vers, err := t.inner.ListVersions(ctx)
	if err != nil {
		return nil, err
	}

	out := []string{}
	seen := map[string]bool{}
	for _, v := range vers {
		if !strings.HasPrefix(v, "ruby-") {
			continue
		}
		ver := strings.TrimPrefix(v, "ruby-")
		if ver == "" || strings.Contains(ver, "-") || seen[ver] {
			continue
		}
		seen[ver] = true
		out = append(out, ver)
	}
	if len(out) == 0 {
		return nil, ErrNoVersions
	}

	// Sort descending semver so [0] == latest.
	svs := make(semver.SemVers, len(out))
	for i, s := range out {
		svs[i] = semver.Parse(s)
	}
	sort.Sort(sort.Reverse(svs))
	for i, s := range svs {
		out[i] = s.Original
	}
	return out, nil
}

func (t *rubyTool) Install(ctx context.Context, version string, destDir string) error {
	v := t.normalizeVersion(version)
	if v == "" || v == "latest" {
		vers, err := t.ListVersions(ctx)
		if err != nil {
			return err
		}
		if len(vers) == 0 {
			return ErrNoVersions
		}
		v = vers[0]
	}
	arts, err := t.ListArtifacts(ctx, v)
	if err != nil {
		return err
	}
	if len(arts) == 0 {
		return ErrNoPlatformArtifact
	}
	artifact := backend.SelectArtifact(arts, runtime.GOOS, runtime.GOARCH, "ruby")
	if artifact == nil {
		return fmt.Errorf("no suitable artifact found for %s/%s for registry:ruby@%s", runtime.GOOS, runtime.GOARCH, v)
	}
	return t.InstallArtifact(ctx, *artifact, destDir)
}

func (t *rubyTool) EnrichLockfile(entry *modfile.RenovateDependency) {
	entry.Provider = "registry"
	if strings.TrimSpace(entry.CurrentValue) == "" {
		entry.CurrentValue = entry.Version
	}
	entry.Versioning = "semver"
}

func (t *rubyTool) ListArtifacts(ctx context.Context, version string) ([]backend.Artifact, error) {
	v := t.normalizeVersion(version)
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

	tag := "ruby-" + v
	at, ok := t.inner.(backend.ArtifactTool)
	if !ok {
		return nil, fmt.Errorf("github tool does not implement ArtifactTool")
	}
	return at.ListArtifacts(ctx, tag)
}

func (t *rubyTool) InstallArtifact(ctx context.Context, artifact backend.Artifact, destDir string) error {
	logger := logging.GetLogger(ctx)
	logger.Warn("ruby (registry provider) is experimental; backed by ruby/ruby-builder prebuilts")

	if err := providerinstall.InstallArtifact(ctx, artifact, destDir, providerinstall.DownloadOptions{}); err != nil {
		return err
	}
	return t.fixRubyShebangs(destDir)
}

func (t *rubyTool) EnsureBinary(ctx context.Context, version string, cmdName string, destDir string) (string, error) {
	if err := t.Install(ctx, version, destDir); err != nil {
		return "", err
	}
	candidates := []string{
		filepath.Join(destDir, "bin", cmdName),
		filepath.Join(destDir, "bin", cmdName+".exe"),
		filepath.Join(destDir, cmdName),
		filepath.Join(destDir, cmdName+".exe"),
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c, nil
		}
	}
	return "", fmt.Errorf("binary %q not found in Ruby installation at %s", cmdName, destDir)
}

func (t *rubyTool) Fix(_ context.Context, destDir string) error {
	return t.fixRubyShebangs(destDir)
}

// --- helpers (as methods to avoid littering package scope) ---

func (t *rubyTool) normalizeVersion(version string) string {
	v := strings.TrimSpace(version)
	v = strings.TrimPrefix(v, "ruby-")
	v = strings.TrimPrefix(v, "Ruby-")
	if v == "" || v == "latest" {
		return v
	}
	return v
}

func (t *rubyTool) fixRubyShebangs(destDir string) error {
	targetRuby := filepath.Join(destDir, "bin", "ruby")
	return filepath.Walk(destDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return nil // skip unreadable
		}
		if !bytes.HasPrefix(b, []byte("#!")) {
			return nil
		}
		// locate end of shebang line
		end := bytes.IndexByte(b, '\n')
		if end == -1 {
			end = len(b)
		}
		shebang := string(b[:end])
		if !strings.Contains(strings.ToLower(shebang), "ruby") {
			return nil
		}
		after := strings.TrimPrefix(shebang, "#!")
		// split at first whitespace to separate interpreter from args
		cut := len(after)
		for i := 0; i < len(after); i++ {
			if after[i] == ' ' || after[i] == '\t' {
				cut = i
				break
			}
		}
		interp := strings.TrimSpace(after[:cut])
		argPart := after[cut:]
		if interp == targetRuby {
			return nil
		}
		// only rewrite if it refers to a ruby interpreter
		base := strings.ToLower(filepath.Base(interp))
		if !strings.HasPrefix(base, "ruby") && !strings.Contains(strings.ToLower(interp), "ruby") {
			return nil
		}
		newShebang := "#!" + targetRuby + argPart
		newContent := newShebang + string(b[end:])
		mode := info.Mode().Perm()
		if mode == 0 {
			mode = 0o755
		}
		return os.WriteFile(path, []byte(newContent), mode)
	})
}
