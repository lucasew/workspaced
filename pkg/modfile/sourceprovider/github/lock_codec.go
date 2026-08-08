package github

import (
	"fmt"
	"net/url"
	"strings"

	"workspaced/pkg/modfile"
)

func (p Provider) ConfigureFromSpec(cfg *modfile.SourceConfig, target string) {
	if cfg == nil {
		return
	}
	cfg.Repo = normalizeGitHubRepo(target)
	cfg.Path = ""
}

func (p Provider) ResolveModuleRef(src modfile.SourceConfig, pathAndVersion string) (fullRef, version string, err error, handled bool) {
	repo := normalizeGitHubRepo(src.Repo)
	if repo == "" {
		return "", "", fmt.Errorf("github source requires repo"), true
	}
	ref, ver := modfile.SplitRefAndVersion(pathAndVersion)
	if ver == "" {
		ver = strings.TrimSpace(src.Ref)
	}
	path := strings.Trim(strings.TrimSpace(ref), "/")
	fullRef = repo
	if path != "" {
		fullRef = repo + "/" + path
	}
	return fullRef, ver, nil, true
}

func (p Provider) RehydrateLockedSource(dep modfile.RenovateDependency) (modfile.LockedSource, bool) {
	if strings.TrimSpace(dep.Kind) != "source" {
		return modfile.LockedSource{}, false
	}
	key := strings.TrimSpace(dep.Ref)
	depName := strings.TrimSpace(dep.DepName)
	packageName := strings.TrimSpace(dep.PackageName)
	datasource := strings.TrimSpace(dep.Datasource)

	repo := ""
	switch {
	case strings.HasPrefix(key, "github:"):
		repo = strings.TrimPrefix(key, "github:")
	case datasource == "git-refs" && depName != "":
		repo = depName
	case strings.Contains(packageName, "github.com/"):
		repo = repoFromGitHubPackageName(packageName)
		if repo == "" {
			repo = depName
		}
	default:
		return modfile.LockedSource{}, false
	}
	if repo == "" {
		return modfile.LockedSource{}, false
	}

	currentValue := strings.TrimSpace(dep.CurrentValue)
	digest := strings.TrimSpace(dep.CurrentDigest)
	out := modfile.LockedSource{
		Provider: "github",
		Repo:     repo,
		Ref:      trackingGitRef(currentValue, ""),
	}
	pin := digest
	if pin == "" && shaRefRe.MatchString(currentValue) {
		pin = currentValue
	}
	out.Hash = pin
	if pin != "" && shaRefRe.MatchString(pin) {
		out.URL = codeloadTarballURL(repo, pin)
	}
	return out, true
}

func (p Provider) LockLookupKeys(lock modfile.LockedSource) []string {
	var keys []string
	repo := normalizeGitHubRepo(lock.Repo)
	if repo == "" && strings.HasPrefix(strings.TrimSpace(lock.Ref), "github:") {
		repo = strings.TrimPrefix(strings.TrimSpace(lock.Ref), "github:")
	}
	if repo == "" {
		return keys
	}
	keys = append(keys, "github:"+repo, repo)
	return keys
}

func (p Provider) CanPersistLock(dep modfile.RenovateDependency, lock modfile.LockedSource) bool {
	_ = lock
	return strings.TrimSpace(dep.DepName) != "" && strings.TrimSpace(dep.Datasource) != ""
}

func (p Provider) LockReusable(locked modfile.LockedSource) bool {
	if strings.TrimSpace(locked.Hash) == "" {
		return false
	}
	ref := strings.TrimSpace(locked.Ref)
	if ref == "" || strings.EqualFold(ref, "HEAD") || shaRefRe.MatchString(ref) {
		return false
	}
	return true
}

func (p Provider) LockMatchesDesired(desired, locked modfile.LockedSource) bool {
	desiredRef := strings.TrimSpace(desired.Ref)
	lockedRef := strings.TrimSpace(locked.Ref)
	if desiredRef == "" || strings.EqualFold(desiredRef, "HEAD") {
		return true
	}
	if desiredRef == lockedRef {
		return true
	}
	if shaRefRe.MatchString(desiredRef) {
		if desiredRef == strings.TrimSpace(locked.Hash) {
			return true
		}
		if pin := refFromCodeloadTarballURL(locked.URL); pin != "" && desiredRef == pin {
			return true
		}
	}
	return false
}

func repoFromGitHubPackageName(packageName string) string {
	packageName = strings.TrimSpace(packageName)
	packageName = strings.TrimPrefix(packageName, "https://")
	packageName = strings.TrimPrefix(packageName, "http://")
	packageName = strings.TrimPrefix(packageName, "github.com/")
	packageName = strings.Trim(packageName, "/")
	if packageName == "" || strings.Contains(packageName, "://") {
		return ""
	}
	parts := strings.Split(packageName, "/")
	if len(parts) < 2 {
		return ""
	}
	return parts[0] + "/" + parts[1]
}

func codeloadTarballURL(repo, pin string) string {
	return "https://codeload.github.com/" + repo + "/tar.gz/" + pin
}

func refFromCodeloadTarballURL(rawURL string) string {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return ""
	}
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	if !strings.EqualFold(parsed.Hostname(), "codeload.github.com") {
		return ""
	}
	parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	if len(parts) != 4 || parts[2] != "tar.gz" {
		return ""
	}
	return strings.TrimSpace(parts[3])
}
