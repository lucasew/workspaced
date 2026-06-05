# Workspaced Code Map

This document is the primary navigation aid for AI agents and humans. Read this before making changes.

## Mental Model (Read First)

Workspaced is a local-first declarative user environment tool.

- Config is CUE-first (`workspaced.cue` + `pkg/configcue`). Templates render from it.
- Everything that touches the host goes through **Drivers** (`pkg/driver`).
- External tools are handled by **Backends** (`pkg/tool/backend`).
- Dotfiles live as **Modules** processed via the lazy source pipeline (`pkg/source` + `pkg/module`).
- All registration (drivers, backends, CLI commands) happens via `init()` + central preludes.

## Key Locations (short)

- `cmd/workspaced/` — CLI (intention-grouped subcommands + generated wiring).
- `pkg/driver/` — Host capability system. Interface per cap + impls + `driver.go` (the generic registry).
- `pkg/tool/backend/` — Tool backends + `Tool` interface. `github/`, `mise/`, `catalog/`.
- `pkg/checks/` — Aggregated checks (`lint/`, `formatter/`).
- `pkg/modfile/` — Lockfile, sources, workspace state.
- `pkg/source/` — Lazy file pipeline + plugins.
- `pkg/configcue/` — CUE loading + schema.
- `pkg/apply/`, `pkg/dotfiles/`, `pkg/deployer/` — Apply flow.
- `pkg/cmdregistry/` — Only for Cobra subcommand wiring.

See full details and "how to find X" recipes below.

## Core Abstractions

- Driver system: `pkg/driver/driver.go` (generic `Register[T]` / `Get[T]`).
- Tool backends: `pkg/tool/backend/backend.go` (`Backend` + `Tool` interfaces).
- Checks: `pkg/checks/check.go`.
- Config + modules: `pkg/configcue`, `pkg/module`, `pkg/source`.
- State: `pkg/modfile`.

## Registration Points

All via `init()` + preludes:

- Drivers: `driver.Register[T](...)` from impls (imported via `pkg/driver/prelude`).
- Tool backends: `tool.Register("github", ...)` (via `pkg/tool/prelude`).
- Curated tools: `catalog.RegisterTool(...)` in `catalog/applications`.
- CLI commands: `GetCommand()` + generated wiring in `cmd/workspaced/*/prelude.go`.

**Rule**: Only import driver/tool preludes from the main `cmd/workspaced/root.go`.

## How to Find Things

- Audio / host feature → `pkg/driver/<feature>/` + matching `cmd/workspaced/driver/<feature>/`.
- New curated tool → `pkg/tool/backend/catalog/applications/`.
- New driver → add impl + import in `pkg/driver/prelude/prelude.go`.
- Lockfile/tool metadata → the `Tool.EnrichLockfile` in the relevant backend.
- Apply logic → `pkg/apply/`, `pkg/source/`, `pkg/deployer/`.

Use `workspaced doctor --verbose` to see active drivers.

## Terminology

See the clean one-sentence definitions in [README.md](README.md).

The project actively reduces "provider" overload:
- Old `pkg/registry` → `pkg/cmdregistry`
- Old `pkg/provider` (checks) → `pkg/checks`
- Tool layer → `pkg/tool/backend` + `Backend` interface + `catalog/`

Prefer explicit phrases in new code/comments.

## Documentation Style This Project Likes

See `TEMPLATES.md` — decision tree + concrete examples + quick reference tables. Do the same for complex subsystems.

`AGENTS.md` contains the "when you do X, touch these files in this order" rules.

## Common Rules & Gotchas

- No lists in module configs (deep merge rules).
- Use `pkg/driver/exec` for process execution outside driver impls.
- Use `fetchurl` driver when you have a hash; `httpclient` otherwise.
- Keep driver preload import centralized.
- `workspaced.lock.json` is the source of truth for resolved tool + module pins.
- Templates use `{{ .Field }}` from CUE-evaluated context.

## Entry Points for Major Flows

- Apply/plan: `cmd/workspaced/home/apply/`, `pkg/apply/engine.go`, `pkg/dotfiles/manager.go`, `pkg/deployer/`
- Tool install: `cmd/workspaced/tool/`, `pkg/tool/manager.go`
- Lazy tool resolution: `pkg/tool/lazy.go`
- Self-update: `cmd/workspaced/selfupdate/`
- Doctor (drivers): `cmd/workspaced/driver/doctor/`

---

**Maintain this file.** When you add a major concept or move a package, update the relevant table/section. A good map is worth more than perfect directory names.
