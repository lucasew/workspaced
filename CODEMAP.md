# Workspaced Code Map

This document is the primary navigation aid for AI agents and humans. Read this before making changes.

## Mental Model (Read This First)

Workspaced is a **local-first declarative user environment tool**.

- **Config is CUE-first**: `workspaced.cue` (user) + `pkg/configcue` (schema + loading). Templates render from the evaluated config.
- **Everything that touches the host is a Driver**: audio, clipboard, WM, notifications, power, etc. Selected dynamically by weight + `CheckCompatibility`.
- **Modules** are the unit of reusable dotfiles. They declare claims (files they own) and are processed with zero intermediate files (in-memory streaming).
- **Tools** manage external binaries (the `tool` subcommand and lazy tools). Specs are `github:owner/repo`, `mise:foo`, or bare curated names (`uv`, `tirith`).
- **Lazy + streaming**: `source.File` and templates delay work until needed.
- **Two registries** (unfortunate name overlap — see below):
  - Driver registry (`pkg/driver`)
  - Tool provider registry (`pkg/tool/provider` + `pkg/tool/tool.go`)

The CLI is built with Cobra + a small `CommandRegistry` so subcommand packages can register themselves without import cycles.

## Directory Responsibilities (Concise)

| Path | Purpose | Key Files / Notes |
|------|---------|-------------------|
| `cmd/workspaced/` | CLI surface. Intention-grouped (see AGENTS.md). | `root.go` (preloads + setup), `driver/prelude.go` (generated), subdirs like `driver/audio/`, `home/apply/`, `tool/` |
| `pkg/driver/` | Pluggable host abstraction system (the biggest architectural win). | `driver.go` (generic `Register[T]`, `Get[T]`, `Doctor`), `prelude/prelude.go` (central import of all impls) |
| `pkg/driver/<cap>/` | One capability (audio, clipboard, notification, ...). | `driver.go` = interface only<br>`facade.go` = convenience funcs that call `driver.Get[Driver](ctx)`<br>`show.go`<br>`<impl>/driver.go` = concrete impl + `init(){ driver.Register[...] }` |
| `pkg/tool/` | External tool/binary manager. | `manager.go`, `resolver.go`, `lazy.go`, `tool.go` (the Register/Get for providers) |
| `pkg/tool/provider/` | Thin "provider" abstraction for tools (interfaces + impls). | `provider.go` (defines `Provider` and `Tool` interfaces — **core**) |
| `pkg/tool/provider/github/` | GitHub Releases backend. | `GitHubTool` implements full `Tool` + `ArtifactTool` |
| `pkg/tool/provider/mise/` | mise backend | |
| `pkg/tool/provider/registry/` | The "registry" provider (maps bare names like "uv" to concrete tools). | `provider.go` (dispatcher), plus `applications/` for ones with custom logic |
| `pkg/tool/provider/registry/applications/` | Curated tools (tirith, claude-code, etc) that often wrap github + filter versions. | Each registers via `registry.RegisterRegistryTool` in init() |
| `pkg/modfile/` | Lockfile model, sources, renovate-style deps, workspace. | Very state-heavy. `workspace.go`, `lockfile.go`, `renovate.go`, `source_ref.go` |
| `pkg/source/` | The lazy rendering pipeline + plugins. | `pipeline.go`, `lazy_file.go`, `plugin_*.go` (dotd, template, module, provider, scanner) |
| `pkg/module/` + `pkg/modulecue/` | Module loading and cue integration. | |
| `pkg/configcue/` | CUE loading, schema, preambles. | `schema.cue`, `loader.go` |
| `pkg/apply/` | The apply engine. | `engine.go` |
| `pkg/dotfiles/` | Dotfiles manager. | `manager.go` |
| `pkg/deployer/` | Execution planner for modules. | `planner.go`, `executor.go` |
| `pkg/checks/` | Base for aggregated "checks" (lint + formatter providers). | Unlike drivers (select 1), these aggregate all that apply. See `lint/`, `formatter/`, and the base in `provider.go` (package checks). |
| `pkg/cmdregistry/` | Cobra subcommand aggregation helper (for modular CLI wiring without cycles). | `command.go` |
| `pkg/parse/spec/` | Tool spec parser (`github:...`, `mise:...`). | |
| `pkg/driver/exec/` | The sanctioned way to run processes (use this instead of `os/exec`). | |

Other notable:
- `pkg/backup/`, `pkg/db/`, `pkg/icons/`, `pkg/palette/`, `pkg/shellgen/`, `pkg/template/`

## Core Abstractions & Their Homes

- **Driver system**: `pkg/driver/driver.go` — generic over interface type using reflection + weights.
- **Tool system**: `pkg/tool/provider/provider.go` — `Provider` (thin factory) + `Tool` (ListVersions + Install + EnrichLockfile). Optional `ArtifactTool`, `BinaryTool`.
- **Command wiring**: `pkg/cmdregistry/command.go` + per-subdir `root.go` that returns a `*cobra.Command` and calls `Registry.FillCommands`.
- **Lint/Format providers**: `pkg/checks/` (base aggregation) + `lint/` and `formatter/` subpackages.
- **Config**: `pkg/configcue` + user `workspaced.cue`.
- **Lock / state for tools & sources**: `pkg/modfile/`.
- **Rendering**: `pkg/source/pipeline.go` + `source.File`.

## Registration / "Magic" Points (Critical for AI)

All use `init()` side effects:

1. **Drivers**: Concrete impl calls `driver.Register[SomeInterface](&Provider{})`. All impl packages are imported from `pkg/driver/prelude/prelude.go`. The root command imports the prelude.
2. **Tool providers**: `tool.Register("github", &Provider{})` etc. Wired via `pkg/tool/prelude/prelude.go`.
3. **Curated registry tools**: `registry.RegisterRegistryTool("tirith", newTirith)` inside the applications packages.
4. **CLI subcommands**: `cmd/workspaced/*/root.go` define `GetCommand()`. A devtool (`pkg/devtools/autoregistry`) scans for them and generates `cmd/workspaced/driver/prelude.go` (and recursively).

**Rule** (from AGENTS.md): Driver and tool preludes are imported **only** from the main root command. Never duplicate the blank imports.

## How to Locate Things Quickly (Recipes)

- "I need to change audio behavior" → `pkg/driver/audio/` (interface + facade) + the impl you care about + `cmd/workspaced/driver/audio/`.
- "Add support for a new tool 'foo'" → If it's a plain github release, just use it. For special version logic, add under `pkg/tool/provider/registry/applications/`.
- "Add a new driver category (e.g. 'bluetooth')" → Create `pkg/driver/bluetooth/driver.go` (interface), `facade.go`, impl dir, add import to `pkg/driver/prelude/prelude.go`, add CLI bits under `cmd/workspaced/driver/bluetooth/`.
- "The lockfile is wrong for a tool" → Look at the specific `Tool.EnrichLockfile` implementation (in github or the curated wrapper), and `pkg/modfile/renovate.go`.
- "Where is the actual file writing?" → `pkg/source/` pipeline + the target module's processing.
- "CUE schema change" → `pkg/configcue/schema.cue` + relevant prelude + Go decoding site.
- "Find all places that call a specific driver" → `driver.Get[SomeDriver](ctx)` (grep for that pattern).

Use `workspaced doctor --verbose` at runtime to see the full interface + provider type paths.

## Term Overloads (Watch Out)

- **provider**:
  - Tool provider (`pkg/tool/provider` — the thin interface for github/mise/etc)
  - Driver provider (the `DriverProvider[T]` interface)
  - (the old `pkg/provider` was renamed to `pkg/checks` to reduce confusion)
- **registry**:
  - `pkg/cmdregistry` (cobra command wiring helper)
  - Tool registry provider (`pkg/tool/provider/registry`)
  - The user's module registry in `workspaced.cue`
- **driver**: always means the host abstraction system in `pkg/driver`.

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
