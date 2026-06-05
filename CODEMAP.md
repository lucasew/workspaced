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
| `pkg/tool/backend/` | Tool backends (the explicit name for what used to be called "tool providers"). | `provider.go` (defines `Backend` and `Tool` interfaces — **core**) |
| `pkg/tool/backend/github/` | GitHub Releases backend. | `GitHubTool` implements full `Tool` + `ArtifactTool` |
| `pkg/tool/backend/mise/` | mise backend | |
| `pkg/tool/backend/catalog/` | The catalog backend (maps bare/curated names like "uv", "tirith" to concrete tools, with possible custom version logic). | `provider.go` (dispatcher) + `applications/` (the curated ones) |
| `pkg/tool/backend/catalog/applications/` | Curated tools with special behavior (tirith filters threatdb versions, etc.). | Register via `catalog.RegisterTool` in their init() |
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
2. **Tool backends**: `tool.Register("github", &Provider{})` (or `&backend{}`) etc. Wired via `pkg/tool/prelude/prelude.go`.
3. **Curated catalog tools**: `catalog.RegisterTool("tirith", newTirith)` inside the applications packages (under catalog).
4. **CLI subcommands**: `cmd/workspaced/*/root.go` define `GetCommand()`. A devtool (`pkg/devtools/autoregistry`) scans for them and generates `cmd/workspaced/driver/prelude.go` (and recursively).

**Rule** (from AGENTS.md): Driver and tool preludes are imported **only** from the main root command. Never duplicate the blank imports.

## How to Locate Things Quickly (Recipes)

- "I need to change audio behavior" → `pkg/driver/audio/` (interface + facade) + the impl you care about + `cmd/workspaced/driver/audio/`.
- "Add support for a new curated tool 'foo'" → Add under `pkg/tool/backend/catalog/applications/` (register with `catalog.RegisterTool`). For plain GitHub, just reference it; no new code needed.
- "Add a new driver category (e.g. 'bluetooth')" → Create `pkg/driver/bluetooth/driver.go` (interface), `facade.go`, impl dir, add import to `pkg/driver/prelude/prelude.go`, add CLI bits under `cmd/workspaced/driver/bluetooth/`.
- "The lockfile is wrong for a tool" → Look at the specific `Tool.EnrichLockfile` implementation (in github or the curated wrapper), and `pkg/modfile/renovate.go`.
- "Where is the actual file writing?" → `pkg/source/` pipeline + the target module's processing.
- "CUE schema change" → `pkg/configcue/schema.cue` + relevant prelude + Go decoding site.
- "Find all places that call a specific driver" → `driver.Get[SomeDriver](ctx)` (grep for that pattern).

Use `workspaced doctor --verbose` at runtime to see the full interface + provider type paths.

## Terminology (Authoritative — use these words)

The project deliberately tries to use precise language to avoid the historical overuse of "provider".

- **Driver**: A pluggable implementation for interacting with a local host capability (audio volume, clipboard copy, notifications, window manager, power, screenshots, terminals, etc.).
  - Always use "driver" for this concept.
  - The registration interface is `DriverProvider[T]` (a thing that can produce an instance of a `Driver` interface after compatibility checks).
  - Concrete code lives in `pkg/driver/<capability>/<impl>/` and registers via `driver.Register[T](...)`.
  - Selection: `driver.Get[SomeDriver](ctx)` picks one (by weight + CheckCompatibility).

- **Tool Backend** (or **Backend**; preferred term — avoid bare "provider"):
  - Something that knows how to list versions and install a specific external CLI tool/binary.
  - The interface lives in `pkg/tool/backend`.
  - Examples: GitHub Releases backend, mise backend, the internal catalog backend.
  - User syntax: `github:owner/repo`, `mise:node`, or bare name (resolved via the catalog).
  - A `Backend` produces `Tool` handles.
  - Registered in `pkg/tool/tool.go` via `tool.Register("github", backend)`.

- **Tool** (or **Managed Tool**): A specific installable package obtained from a backend. Implements `ListVersions` + `Install` + `EnrichLockfile`. Optional `ArtifactTool` / `BinaryTool`.

- **Catalog** (or Curated Catalog): The backend that maps short names ("uv", "tirith", "claude-code") to concrete GitHub-based tools, sometimes with custom version filtering (see `pkg/tool/backend/catalog`).

- **Check** / **Checks**:
  - Base concept for discoverable, directory-applicable actions that produce side effects or reports (linters and formatters).
  - Package: `pkg/checks`.
  - Base interface: `Check` (Name + Detect).
  - Extended by `Linter` (produces SARIF) and `Formatter`.
  - All matching checks are *aggregated* (unlike drivers, which select one).

- **Module**:
  - A reusable, claim-declaring unit of configuration (user's `modules/` or built-in).
  - Resolved by a `module.Provider` (core, local, etc.).

- **Source** / **Source Provider**:
  - For resolving module references like `github:foo/bar` or local paths.
  - See `pkg/modfile` source providers and `pkg/source`.

- **Registry** (use qualified):
  - `cmdregistry`: The small helper for wiring Cobra subcommands (`pkg/cmdregistry`).
  - Module registry: user's `workspaced` config declaring modules.
  - Avoid bare "registry" for tool backends.

**Current state of cleanup (as of this doc):**
- Old bare `pkg/registry` → `pkg/cmdregistry`
- Old bare `pkg/provider` (lint/format) → `pkg/checks`
- Tool "provider" concept → `pkg/tool/backend` + `Backend` interface + `catalog` for the curated short names. "provider" word is now mostly confined to concrete implementation structs inside their packages (e.g. `github.Provider`).

When writing code or comments, prefer the long form on first use: "the GitHub Releases tool backend", "the PulseAudio driver implementation", "the Ruff linter check".

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
