# Workspaced code map

Read this before editing.

## Architecture

CUE config (`workspaced.cue`) drives everything.

- Drivers (`pkg/driver`): OS features (audio, clipboard, WM, …). One impl per interface, chosen by weights and compatibility checks.
- Modules + source pipeline (`pkg/module`, `pkg/source`): config and templates become real files, streamed in memory.
- Tool backends (`pkg/tool/backend`): github, mise, catalog. Each yields installable, lockable tools.
- Checks (`pkg/checks`): matching linters and formatters all run.
- CLI packages under `cmd/workspaced/` are small and intention-based.

## Critical locations

- `pkg/driver/driver.go` and `pkg/driver/prelude`: driver system
- `pkg/tool/backend/backend.go` and `catalog/`: tool backends
- `pkg/tool/checks`: optional `InstallChecker` for install trees. `mise run test:registry-install` (sets `WORKSPACED_TEST_TOOL_INSTALL=1`) does full install verification; required by `mise release`, and on CI for `refs/tags/*` (via `CI`+`GITHUB_REF` or the autorelease workflow step)
- `pkg/configcue/`, `pkg/modfile/`, `pkg/source/`: config, state, rendering
- `pkg/apply/`, `pkg/deployer/`: apply flow
- `pkg/taskgroup/`: Session, progress UI, `Map`/`Each`/`Isolate` (package doc + AGENTS.md map/reduce rule)
- `cmd/workspaced/root.go`: only place that imports driver/tool preludes

## Registration (`init()` based)

- Drivers: `driver.Register[T](impl)`
- Backends: `tool.Register("github", impl)`
- Curated tools: `catalog.RegisterTool(name, ctor)`
- CLI subcommands: `GetCommand()` (wired by devtool)

Never import the preludes except from `cmd/workspaced/root.go`.

## Common tasks

- New driver capability: `pkg/driver/newthing/` (interface + facade + impl), then add to prelude.
- Curated tool: `pkg/tool/backend/catalog/applications/`
- Change how a tool locks: that backend's `EnrichLockfile`.
- CUE schema changes: `pkg/configcue/schema.cue`

Rules live in AGENTS.md. An older long-table version of this file may still be in git history if you need it.

## Terminology

See README.md (one sentence per term). Prefer factory (drivers), backend (tools), check (linters/formatters). Leftover "provider" wording is mostly module sources (`SourceProvider`) and transitional tool backend types.

## Doc style

For complex subsystems, mirror `skills/workspaced/references/templates.md`: decision tree, concrete examples, short reference tables. Usage skill material goes under `skills/workspaced/`.

`AGENTS.md` has the "when you do X, touch these files in this order" lists.

## Gotchas (also in AGENTS.md)

- No lists in module configs.
- Use `pkg/driver/exec` outside driver implementations.
- Import driver/tool preludes only from `cmd/workspaced/root.go`.

Rest is in AGENTS.md and `skills/workspaced/`.

Update this file when structure shifts in a big way.
