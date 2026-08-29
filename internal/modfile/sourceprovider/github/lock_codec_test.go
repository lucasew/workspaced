package github

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/lucasew/workspaced/internal/modfile"
)

func TestRehydrateLockedSource(t *testing.T) {
	t.Parallel()
	p := Provider{}
	dep := modfile.RenovateDependency{
		Kind:          "source",
		Ref:           "github:PapirusDevelopmentTeam/papirus-icon-theme",
		DepName:       "PapirusDevelopmentTeam/papirus-icon-theme",
		PackageName:   "https://github.com/PapirusDevelopmentTeam/papirus-icon-theme",
		CurrentValue:  "master",
		CurrentDigest: "702499f331aa9c38309e1af99de4021013916297",
		Datasource:    "git-refs",
	}
	lock, ok := p.RehydrateLockedSource(dep)
	if !ok {
		t.Fatal("expected github provider to own row")
	}
	if lock.Provider != "github" || lock.Repo != "PapirusDevelopmentTeam/papirus-icon-theme" {
		t.Fatalf("identity: %#v", lock)
	}
	if lock.Ref != "master" {
		t.Fatalf("Ref = %q", lock.Ref)
	}
	if lock.Hash != "702499f331aa9c38309e1af99de4021013916297" {
		t.Fatalf("Hash = %q", lock.Hash)
	}
	wantURL := "https://codeload.github.com/PapirusDevelopmentTeam/papirus-icon-theme/tar.gz/702499f331aa9c38309e1af99de4021013916297"
	if lock.URL != wantURL {
		t.Fatalf("URL = %q", lock.URL)
	}
	if !p.LockReusable(lock) {
		t.Fatal("expected reusable")
	}
	if _, ok := p.RehydrateLockedSource(modfile.RenovateDependency{Kind: "tool", Ref: "github:cli/cli"}); ok {
		t.Fatal("must not own tool rows")
	}
}

func TestLockMatchesDesired(t *testing.T) {
	t.Parallel()
	p := Provider{}
	locked := modfile.LockedSource{
		Provider: "github",
		Repo:     "PapirusDevelopmentTeam/papirus-icon-theme",
		Ref:      "master",
		Hash:     "702499f331aa9c38309e1af99de4021013916297",
		URL:      "https://codeload.github.com/PapirusDevelopmentTeam/papirus-icon-theme/tar.gz/702499f331aa9c38309e1af99de4021013916297",
	}
	if !p.LockMatchesDesired(modfile.LockedSource{}, locked) {
		t.Fatal("empty desired")
	}
	if !p.LockMatchesDesired(modfile.LockedSource{Ref: "HEAD"}, locked) {
		t.Fatal("HEAD desired")
	}
	if !p.LockMatchesDesired(modfile.LockedSource{Ref: "master"}, locked) {
		t.Fatal("same branch")
	}
	if p.LockMatchesDesired(modfile.LockedSource{Ref: "develop"}, locked) {
		t.Fatal("other branch")
	}
	if !p.LockMatchesDesired(modfile.LockedSource{Ref: "702499f331aa9c38309e1af99de4021013916297"}, locked) {
		t.Fatal("pin sha")
	}
	if p.LockReusable(modfile.LockedSource{Provider: "github", Hash: "abc", Ref: "702499f331aa9c38309e1af99de4021013916297"}) {
		t.Fatal("sha-only tracking must not be reusable")
	}
}

func TestUpsertSourceIdempotentAfterReload(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	sumPath := filepath.Join(dir, "workspaced.lock.json")
	sum := &modfile.SumFile{}
	entry := modfile.LockedSource{
		Provider: "github",
		Repo:     "PapirusDevelopmentTeam/papirus-icon-theme",
		Ref:      "master",
		Hash:     "702499f331aa9c38309e1af99de4021013916297",
		URL:      "https://codeload.github.com/PapirusDevelopmentTeam/papirus-icon-theme/tar.gz/702499f331aa9c38309e1af99de4021013916297",
	}
	if !sum.EnsureSource("papirus", entry) {
		t.Fatal("expected initial ensure to change")
	}
	if err := os.WriteFile(sumPath, []byte(`{
  "dependencies": [
    {
      "kind": "source",
      "ref": "github:PapirusDevelopmentTeam/papirus-icon-theme",
      "depName": "PapirusDevelopmentTeam/papirus-icon-theme",
      "packageName": "https://github.com/PapirusDevelopmentTeam/papirus-icon-theme",
      "currentValue": "master",
      "currentDigest": "702499f331aa9c38309e1af99de4021013916297",
      "datasource": "git-refs"
    }
  ]
}
`), 0o644); err != nil {
		t.Fatal(err)
	}
	loaded, err := modfile.LoadSumFile(sumPath)
	if err != nil {
		t.Fatal(err)
	}
	lock, ok := loaded.FindSource("github:PapirusDevelopmentTeam/papirus-icon-theme")
	p := Provider{}
	if !ok || !p.LockReusable(lock) {
		t.Fatalf("reloaded lock not reusable: ok=%v %#v", ok, lock)
	}
	if _, ok := loaded.FindSource("PapirusDevelopmentTeam/papirus-icon-theme"); !ok {
		t.Fatal("missing lookup by repo key")
	}
	reuse := entry
	reuse.Hash = lock.Hash
	reuse.URL = lock.URL
	reuse.Ref = lock.Ref
	if loaded.UpsertSource("papirus", reuse) {
		t.Fatalf("expected idempotent upsert, deps=%#v", loaded.Dependencies)
	}
}

func TestConfigureFromSpecAndModuleRef(t *testing.T) {
	t.Parallel()
	p := Provider{}
	cfg := modfile.SourceConfig{Provider: "github"}
	p.ConfigureFromSpec(&cfg, "owner/repo")
	if cfg.Repo != "owner/repo" || cfg.Path != "" {
		t.Fatalf("cfg=%#v", cfg)
	}
	fullRef, ver, err, handled := p.ResolveModuleRef(cfg, "subdir@v1")
	if !handled || err != nil {
		t.Fatalf("handled=%v err=%v", handled, err)
	}
	if fullRef != "owner/repo/subdir" || ver != "v1" {
		t.Fatalf("fullRef=%q ver=%q", fullRef, ver)
	}
}

func TestLockLookupKeys(t *testing.T) {
	t.Parallel()
	p := Provider{}
	keys := p.LockLookupKeys(modfile.LockedSource{Repo: "o/r"})
	if len(keys) != 2 || keys[0] != "github:o/r" || keys[1] != "o/r" {
		t.Fatalf("keys=%v", keys)
	}
}

func TestCanPersistLock(t *testing.T) {
	t.Parallel()
	p := Provider{}
	lock := modfile.LockedSource{Provider: "github", Repo: "o/r"}
	if p.CanPersistLock(modfile.RenovateDependency{Ref: "deadbeef"}, lock) {
		t.Fatal("incomplete row must be rejected")
	}
	if !p.CanPersistLock(modfile.RenovateDependency{DepName: "o/r", Datasource: "git-refs"}, lock) {
		t.Fatal("complete row must persist")
	}
}

func TestRefFromCodeloadTarballURL(t *testing.T) {
	t.Parallel()
	if got := refFromCodeloadTarballURL("https://codeload.github.com/o/r/tar.gz/abc1234"); got != "abc1234" {
		t.Fatalf("got %q", got)
	}
	if got := refFromCodeloadTarballURL("https://example.com/o/r/tar.gz/abc"); got != "" {
		t.Fatalf("non-codeload got %q", got)
	}
}
