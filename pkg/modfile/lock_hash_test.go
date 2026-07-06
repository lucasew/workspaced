package modfile

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"

	"workspaced/pkg/logging"
	"workspaced/pkg/taskgroup"
)

type stubHashProvider struct {
	id    string
	calls atomic.Int64
}

func (p *stubHashProvider) ID() string { return p.id }

func (p *stubHashProvider) ResolvePath(ctx context.Context, alias string, src SourceConfig, rel string, modulesBaseDir string) (string, error) {
	return "", fmt.Errorf("unused")
}

func (p *stubHashProvider) LockHash(ctx context.Context, alias string, src SourceConfig, modulesBaseDir string) (string, SourceConfig, error) {
	p.calls.Add(1)
	src.Ref = "main"
	src.URL = "https://example.test/" + alias
	return fmt.Sprintf("sha256:%s", alias), src, nil
}

func (p *stubHashProvider) Normalize(src SourceConfig) SourceConfig {
	src.Provider = p.id
	return src
}

func (p *stubHashProvider) EnrichRenovateDependency(dep *RenovateDependency, src LockedSource) {}
func (p *stubHashProvider) ConfigureFromSpec(cfg *SourceConfig, target string)                  {}
func (p *stubHashProvider) ResolveModuleRef(src SourceConfig, pathAndVersion string) (string, string, error, bool) {
	return "", "", nil, false
}
func (p *stubHashProvider) RehydrateLockedSource(dep RenovateDependency) (LockedSource, bool) {
	return LockedSource{}, false
}
func (p *stubHashProvider) LockLookupKeys(lock LockedSource) []string { return nil }
func (p *stubHashProvider) CanPersistLock(dep RenovateDependency, lock LockedSource) bool {
	return true
}
func (p *stubHashProvider) LockReusable(locked LockedSource) bool {
	return locked.Hash != ""
}
func (p *stubHashProvider) LockMatchesDesired(desired, locked LockedSource) bool { return true }

func registerStubHashProvider(t *testing.T, id string) *stubHashProvider {
	t.Helper()
	p := &stubHashProvider{id: id}
	prev, hadPrev := sourceProviders[id]
	RegisterSourceProvider(p)
	t.Cleanup(func() {
		if hadPrev {
			sourceProviders[id] = prev
			return
		}
		delete(sourceProviders, id)
	})
	return p
}

func testGroupCtx(t *testing.T) (*taskgroup.Group, context.Context) {
	t.Helper()
	g, ctx := taskgroup.New(logging.NewWriterContext(t.Output()), taskgroup.DefaultLimits())
	t.Cleanup(func() { _ = g.Wait() })
	return g, ctx
}

func TestPopulateSourceLockHashesSkipsExisting(t *testing.T) {
	t.Parallel()
	p := registerStubHashProvider(t, "stubhash-skip-existing")
	_, ctx := testGroupCtx(t)

	mod := &ModFile{Sources: map[string]SourceConfig{
		"alpha": {Provider: p.id, Repo: "o/alpha"},
		"beta":  {Provider: p.id, Repo: "o/beta"},
	}}
	entries := map[string]LockedSource{
		"alpha": {Provider: p.id, Repo: "o/alpha"},
		"beta":  {Provider: p.id, Repo: "o/beta", Hash: "sha256:keep"},
	}

	if err := PopulateSourceLockHashes(ctx, mod, t.TempDir(), entries); err != nil {
		t.Fatalf("PopulateSourceLockHashes: %v", err)
	}
	if got := p.calls.Load(); got != 1 {
		t.Fatalf("calls=%d want 1 (beta already hashed)", got)
	}
	if entries["alpha"].Hash != "sha256:alpha" {
		t.Fatalf("alpha hash=%q", entries["alpha"].Hash)
	}
	if entries["alpha"].Ref != "main" || entries["alpha"].URL == "" {
		t.Fatalf("alpha missing resolved metadata: %+v", entries["alpha"])
	}
	if entries["beta"].Hash != "sha256:keep" {
		t.Fatalf("beta hash changed to %q", entries["beta"].Hash)
	}
}

func TestPopulateSourceLockHashesParallel(t *testing.T) {
	t.Parallel()
	p := registerStubHashProvider(t, "stubhash-parallel")
	_, ctx := testGroupCtx(t)

	mod := &ModFile{Sources: map[string]SourceConfig{
		"one": {Provider: p.id, Repo: "o/one"},
		"two": {Provider: p.id, Repo: "o/two"},
	}}
	entries := map[string]LockedSource{
		"one": {Provider: p.id, Repo: "o/one"},
		"two": {Provider: p.id, Repo: "o/two"},
	}

	if err := PopulateSourceLockHashes(ctx, mod, t.TempDir(), entries); err != nil {
		t.Fatalf("PopulateSourceLockHashes: %v", err)
	}
	if got := p.calls.Load(); got != 2 {
		t.Fatalf("calls=%d want 2", got)
	}
	if entries["one"].Hash != "sha256:one" || entries["two"].Hash != "sha256:two" {
		t.Fatalf("entries=%+v", entries)
	}
}

func TestPopulateSourceLockHashesSkipsWhenComplete(t *testing.T) {
	t.Parallel()
	p := registerStubHashProvider(t, "stubhash-skip-all")
	_, ctx := testGroupCtx(t)

	mod := &ModFile{Sources: map[string]SourceConfig{
		"done": {Provider: p.id, Repo: "o/done"},
	}}
	entries := map[string]LockedSource{
		"done": {Provider: p.id, Repo: "o/done", Hash: "sha256:done"},
	}
	if err := PopulateSourceLockHashes(ctx, mod, t.TempDir(), entries); err != nil {
		t.Fatalf("PopulateSourceLockHashes: %v", err)
	}
	if got := p.calls.Load(); got != 0 {
		t.Fatalf("calls=%d want 0", got)
	}
}

func TestPopulateSourceLockHashesUnsupportedProvider(t *testing.T) {
	t.Parallel()
	_, ctx := testGroupCtx(t)
	mod := &ModFile{Sources: map[string]SourceConfig{
		"x": {Provider: "no-such-provider"},
	}}
	entries := map[string]LockedSource{
		"x": {Provider: "no-such-provider"},
	}
	err := PopulateSourceLockHashes(ctx, mod, t.TempDir(), entries)
	if err == nil {
		t.Fatal("expected error")
	}
}
