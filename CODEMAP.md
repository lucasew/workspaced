# Workspaced code map

Read this before editing. Layout rule is in AGENTS.md (`pkg/` vs `internal/` vs `cmd/`).

## Architecture

CUE config (`workspaced.cue`) drives everything.

- Drivers (`pkg/driver`): OS features (audio, clipboard, WM, …). One impl per interface, chosen by weights and compatibility checks.
- Modules + source pipeline (`internal/module`, `internal/source`): config and templates become real files, streamed in memory.
- Tool backends (`internal/tool/backend`): github, mise, catalog. Each yields installable, lockable tools.
- Checks (`internal/checks`): matching linters and formatters all run.
- CLI packages under `cmd/workspaced/` are small and intention-based.

## Critical locations

- `pkg/driver/driver.go` and `pkg/driver/prelude`: driver system
- `pkg/taskgroup/`: Session, progress UI, `Map`/`Each`/`Isolate` (package doc + AGENTS.md map/reduce rule)
- `pkg/palette/`, `pkg/logging/`, `pkg/api/`: rest of `pkg/`
- `internal/tool/backend/backend.go` and `internal/tool/backend/catalog/`: tool backends
- `internal/tool/checks`: optional `InstallChecker` for install trees. `mise run test:registry-install` (sets `WORKSPACED_TEST_TOOL_INSTALL=1`) does full install verification; required by `mise release`. Multi-target CI: `.github/workflows/registry-install.yml` (linux amd64 + arm64). Per-tool failures append to `GITHUB_STEP_SUMMARY` from `install_test.go` when that env is set.
- `internal/configcue/`, `internal/modfile/`, `internal/source/`: config, state, rendering
- `internal/apply/`, `internal/deployer/`: apply flow
- `cmd/workspaced/root.go`: `pkg/driver/prelude`; tool/check preludes load from the cmds that need them

## Registration (`init()` based)

- Drivers: `driver.Register[T](impl)`
- Backends: `tool.Register("github", impl)`
- Curated tools: `catalog.RegisterTool(name, ctor)`
- CLI subcommands: `GetCommand()` (wired by devtool)

Never import driver prelude except from `cmd/workspaced/root.go`. Tool/check preludes: from the cmd that needs them, not from `pkg/`.

## Common tasks

- New driver capability: `pkg/driver/newthing/` (interface + facade + impl), then add to prelude.
- Curated tool: `internal/tool/backend/catalog/applications/`
- Change how a tool locks: that backend's `EnrichLockfile`.
- CUE schema changes: `internal/configcue/schema.cue`

Rules live in AGENTS.md. An older long-table version of this file may still be in git history if you need it.

## Terminology

See README.md (one sentence per term). Prefer factory (drivers), backend (tools), check (linters/formatters). Leftover "provider" wording is mostly module sources (`SourceProvider`) and transitional tool backend types.

## Doc style

For complex subsystems, mirror `skills/workspaced/references/templates.md`: decision tree, concrete examples, short reference tables. Usage skill material goes under `skills/workspaced/`.

`AGENTS.md` has the "when you do X, touch these files in this order" lists.

## Gotchas (also in AGENTS.md)

- No lists in module configs.
- Use `pkg/driver/exec` outside driver implementations.
- Import driver prelude only from `cmd/workspaced/root.go`.

Rest is in AGENTS.md and `skills/workspaced/`.

Update this file when structure shifts in a big way.
