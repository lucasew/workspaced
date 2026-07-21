# `--no-cache` / `WORKSPACED_NO_CACHE`

Full-cascade cold materialization. Ignore warm install/module/source/shell
caches and treat deploy noops as updates so a new binary/engine can rewrite
derived state without hand-bumped per-behavior versions.

## Switch

| Surface | Form |
|---------|------|
| Global flag | `workspaced --no-cache …` |
| Env | `WORKSPACED_NO_CACHE` |

Same bit. Env is **on** when set and not `0` / `false` / `no` / `off`
(case-insensitive). Empty is off. Flag OR env arms the bit.

Wired once in root `PersistentPreRun` into `cmdctx` (same pattern as dry-run /
verbose). Layers only call `cmdctx.IsNoCache(ctx)` — no scattered `os.Getenv`.

On arm: one **info** log at start (`no-cache enabled …`). Per-site misses stay
**debug** (`-v`).

## Not this switch

| Switch | Scope |
|--------|--------|
| `utils shell init --force` | Shell-init cache file only (unchanged) |
| `mod lock` / tidy / renovate | Dependency pin refresh — not cache cold-start |
| Per-behavior engine version salts | Out of scope for v1; no-cache is the blunt lever |

## Cascade (v1: all or nothing)

When armed and **not** dry-run:

| Layer | Hit rule | Miss / rewrite |
|-------|----------|----------------|
| Deploy planner | n/a | Existing targets that would be `noop` become **`update`** (skip bundle fast-path and content-equality noop) |
| Module gen caches (e.g. icons) | `exists && !no-cache` | Regenerate into temp → **atomic dir swap** → repopulate |
| Source fetch cache | `exists && !no-cache` | Re-fetch into temp → **atomic dir swap** → repopulate |
| Tools | version dir non-empty && checks pass && `!no-cache` | Re-fetch **lock pins** (no re-resolve of `latest` beyond normal Ensure rules) into temp → **atomic dir swap** |
| Shell init | cache file exists && `!force && !no-cache` | Rebuild; write via temp file rename (repopulate) |

When armed **and** dry-run / `plan`:

- Planner still widens (noops → updates in the plan).
- Deploy executor does **not** write managed home/repo files (`ApplyOptions.DryRun`
  / root `--dry-run`).
- Materializers honor `cmdctx.IsDryRun` on the task context: skip re-fetch/swap
  if a warm artifact exists. Root `--dry-run` is set before session `Enter`.
  `home plan` / `codebase plan` call `Session.Overlay` after forcing dry-run so
  tasks see the same bit (no network rematerialization when warm under
  `--no-cache`).

## Tools detail

- **Ignore “already installed”** when no-cache.
- **Pins stay**: lockfile versions are not refreshed by no-cache.
- **Atomic swap**: install to a sibling temp dir, then replace the live version
  dir (rename-aside + rename-in, remove aside). No half-written tree left as the
  live path after success.

## Materializer pattern

Every skip path:

```text
if artifactExists && !cmdctx.IsNoCache(ctx) {
  return hit
}
if cmdctx.IsDryRun(ctx) {
  // log debug: would rematerialize; return existing if any
}
// write to temp → atomic swap into place (repopulate for next warm run)
```

`no-cache` means do not **trust** what was there; still **write** fresh caches
after a successful run.

## Dry-run interaction

`cmdctx.IsDryRun` + no-cache ⇒ **widen plan only**. No network materialization,
no swaps, no deploy writes (executor already skipped under dry-run apply).

## Out of scope (iterate later)

- Partial cascade flags (tools-only, modules-only, …)
- Automatic per-behavior version constants in fingerprints
- Binary `GetBuildID()` folded into every cache key by default
