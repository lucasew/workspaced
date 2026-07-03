# Workspaced Code Map (short reference for agents)

Read this before editing.

## One-paragraph architecture

CUE config (`workspaced.cue`) drives everything.

- **Drivers** (`pkg/driver`): every OS feature (audio, clipboard, WM...). One is chosen per interface via weights + compatibility.
- **Modules** + **source pipeline** (`pkg/module`, `pkg/source`): turn config + templates into actual files with zero intermediate artifacts.
- **Tool Backends** (`pkg/tool/backend`): github / mise / catalog. Each produces Tools that can be installed and locked.
- **Checks** (`pkg/checks`): linters and formatters are aggregated (all that match run).
- CLI is built from small intention-based packages under `cmd/workspaced/`.

## Critical locations

- `pkg/driver/driver.go` + `pkg/driver/prelude` — the driver system
- `pkg/tool/backend/backend.go` + `catalog/` — tool backends
- `pkg/configcue/`, `pkg/modfile/`, `pkg/source/` — config + state + rendering
- `pkg/apply/`, `pkg/deployer/` — the apply flow
- `cmd/workspaced/root.go` — only place that imports driver/tool preludes

## Registration (all init() based)

- Drivers: `driver.Register[T](impl)`
- Backends: `tool.Register("github", impl)`
- Curated tools: `catalog.RegisterTool(name, ctor)`
- CLI subcommands: `GetCommand()` pattern (auto-wired by devtool)

Rule: never import the preludes anywhere except `cmd/workspaced/root.go`.

## How to do common things

- Add driver for new capability → `pkg/driver/newthing/` (interface + facade + impl) + add to prelude.
- Add curated tool → `pkg/tool/backend/catalog/applications/`
- Change how a tool is locked → its `EnrichLockfile` in the backend.
- Touch CUE schema → `pkg/configcue/schema.cue`

See AGENTS.md for rules. See full previous version of this file if you need the long table.

## Terminology

See README.md (one sentence per term). Prefer **factory** (drivers), **backend** (tools), **check** (linters/formatters). Remaining "provider" wording is mostly module sources (`SourceProvider`) and transitional tool backend types.

## Documentation Style This Project Likes

See `skills/workspaced/references/templates.md` — decision tree + concrete examples + quick reference tables. Do the same for complex subsystems (usage skill under `skills/workspaced/`).

`AGENTS.md` contains the "when you do X, touch these files in this order" rules.

## Common gotchas (from AGENTS.md)

- No lists in module configs.
- Use `pkg/driver/exec` outside of driver implementations.
- Import driver/tool preludes only from `cmd/workspaced/root.go`.

See AGENTS.md and `skills/workspaced/` for the rest.

**Update this file when big structure changes happen.**
