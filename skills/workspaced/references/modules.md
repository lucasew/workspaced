# Modules and inputs

**Essential:** model, lock vs cue, gotchas. **Deep:** cue examples beyond init,
on-disk layout detail. Universals (cue/lock/plan): **SKILL.md** — do not
re-expand them here.

## Model (essential)

A **module** is a unit of environment contribution: on-disk content (static
files, templates, providers) plus a **`config`** object from cue that templates
and module logic can consume.

An **input** names **where module source comes from** (local tree, git, other
providers). Modules point at an input (and often a `path` within it).

Rough flow:

```text
cue: inputs + modules.<name> { input, path, config, … }
        │
        ├── resolve sources ──► workspaced.lock.json (pins, hashes, …)
        │
        └── enabled modules ──► source pipeline ──► plan/apply
```

Modules are **not** tools (binaries) and **not** drivers (live OS verbs). They
are the main vehicle for “make my home/repo files look like this.”

## Cue side (roles, not one style)

Typical roles you’ll see (names may vary with your file; inspect theirs):

| Role | Meaning |
|------|---------|
| **inputs** | Named sources (`self`, remote specs, versions/refs as applicable) |
| **modules.\<id\>** | Enable/bind a module: which input, path inside input, `config` bag |
| **config on module** | Data for that module’s templates/behavior — your playground in CUE |

Init starter pattern (illustrative only):

```cue
workspaced: modules: example: {
	input: "self"
	path:  "modules/example"
	config: {
		enable:   true
		greeting: "Hello from workspaced!"
	}
}
```

`input: "self"` + `path: "modules/example"` usually means “this module lives in
my dotfiles/workspace tree under that path.” Remote inputs use provider-style
specs and rely more on lock pins.

Author modules with as much CUE structure as you want (`cue.md`). Workspaced
needs the **evaluated** module entries it knows how to run.

## Lockfile

**`workspaced.lock.json`** records resolved pins for enabled/relevant sources
(and related tool/source metadata). Apart from identifiers like kind/ref, many
fields exist to help automation (e.g. Renovate) as much as humans.

| | **workspaced.cue** | **workspaced.lock.json** |
|---|--------------------|---------------------------|
| **You edit for intent** | Yes | Rarely |
| **Refreshed by** | Your editor / agent | `mod lock` / `mod tidy` |
| **Holds** | What modules/inputs/config you want | What versions/hashes were resolved |

`mod tidy` is an **alias for `mod lock`** in this codebase — both refresh lock
for the detected workspace.

After changing inputs, module sources, or versions in cue, run lock refresh
**in that workspace** before expecting apply to honor the new resolution.

## On-disk module layout (user view)

A module directory typically contains things like `config/` (files and
templates destined for targets), maybe module-local cue/docs, and whatever
providers expect. The example module under init templates is the best small
tour.

Template **kinds** (static, `.tmpl`, multi-file, `.d.tmpl`) are covered in
`templates.md`.

## Config bag vs files

- **`config` in cue** — parameters (`enable`, greetings, feature flags, maps of
  apps, …). Consumed as template/module data (e.g. `.module.*` in example
  templates).
- **Files under the module** — what gets linked/rendered/placed by apply.

Enabling a module without understanding its templates/config is fine; overriding
behavior is usually more `config` (and cue creativity) than forking files, until
you need new artifacts.

## Merge and module `config`

Module configuration is composed in a **deep-merge-friendly** world. At those
merge boundaries, **prefer keyed objects** over lists that multiple layers might
each try to own. You can still author lists in CUE and project them to maps
before the merge boundary (`cue.md`).

## Operations (verbs only)

| Verb | Role |
|------|------|
| Edit cue modules/inputs | Declare intent |
| Edit module tree | Change artifacts/templates |
| `mod lock` / `mod tidy` | Refresh lock pins |
| `home`/`codebase` `plan` | See what would change |
| `home`/`codebase` `apply` | Converge |

Flags and extras: `--help` on each.

## Gotchas

- **Cue changed, lock not refreshed** — apply/plan may use old pins or fail
  resolution for remote/self inputs that moved.
- **Module not enabled / wrong `input` or `path`** — files exist on disk but
  workspaced never selects that module; “template not applied.”
- **`self` path wrong** — off-by-one directory (`modules/foo` vs `foo`) is
  common after moves.
- **Treating lock as source of truth** — edits get overwritten or disagree with
  cue; fix cue/inputs then lock.
- **List-shaped module config at merge points** — surprising replace/drop when
  more than one contributor merges; use maps or CUE projection (`cue.md`).
- **Confusing module with tool** — installing `ripgrep` doesn’t add a module;
  adding a module doesn’t put `rg` on PATH unless the module/templates/shims
  do that separately.
- **Wrong workspace for `mod lock`** — refreshed lock in tree A, apply in tree B.
- **Remote input without accepting lock/hash flow** — first enable may need lock
  generation and network; offline apply won’t invent pins.
