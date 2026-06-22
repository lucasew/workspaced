# Tools and access

**Essential:** access modes + gotchas. **Deep / live:** `workspaced tool --help`
and `workspaced tool with --help` (spec formats, examples, resolution details).
Do not copy help Examples into this file.

## Model (essential)

A **tool** is a versioned program obtained through a **backend** (registry/catalog,
`github:…`, `mise:…`, …). Workspaced can install into a store, expose **shims**,
run one-shot versions via **`tool with`**, and **lazy-ensure** tools internally
(lint/format/etc.).

Tools ≠ modules (files/state) ≠ drivers (live OS).

## Access modes (essential)

| Mode | Persistence | Typical use |
|------|-------------|-------------|
| **`tool with <specs>… -- <cmd>…`** | **That invocation only** | One-shot / pin versions for a single command |
| **`tool install`** (and normal install flows) | On disk in tool store | Want it available via workspaced going forward |
| **Shims** (`…/workspaced/shims` on PATH) | Until shim/PATH removed | Daily `rg` / `uv` in an interactive shell |
| **Lazy / internal ensure** | As side effect of other cmds | Codebase lint/format pulling a tool |
| **`open` / mise helpers** | Depends | Launchers, mise-oriented entry — not the same as `with` |

**Core surprise:** `tool with` is **not** “fix my shell PATH.” If the next
terminal lacks the tool, that is expected unless shims + shell init (e.g.
`utils shell init` / dotfiles) already provide it.

## Layout (essential, orientation)

Under `~/.local/share/workspaced/` conceptually: **`tools/`** (store),
**`shims/`** (PATH entries). Install without shims/PATH ⇒ “installed but my
shell doesn’t see it.”

## `tool with` (essential model only)

Full argv/spec grammar and examples: **`workspaced tool with --help`**.

Skill-only points worth remembering:

- Specs **before** `--`, command **after** (prefer always using `--`).
- Ensures/installs missing tools — **side effects / network**, not a dry probe.
- Multiple tools allowed; binary resolution is not “only the last tool matters
  for everything” — help describes the rule; don’t second-guess here.
- **Ephemeral** access mode; combine with install/shims for day-to-day.

Backends at user level: bare/curated names often **registry**; languages often
**`mise:`**; repos often **`github:`**. If a bare name fails, try explicit
backend or `tool search` (see help).

## Other verbs (names only)

`search`, `list`, `install`, `which`, `versions`, `latest`, `artifacts`, `with`
— roles and flags from `workspaced tool --help` / subcommand help.

## Lock / cue (pointer)

Tool pins may appear in lock/automation flows; that’s still **pins**, not a
substitute for access modes. Cue/lock intent vs pins: **SKILL universals** +
`modules.md`. Don’t restate lock philosophy here.

## Gotchas (essential)

- **`with` ≠ permanent PATH** — see access modes.
- **Missing `--`** — legacy/ambiguous; always use `--`.
- **Wrong backend for the name** — mise vs registry vs github; help + search.
- **Shims not on PATH** — store has it, shell doesn’t; shell init / dotfiles.
- **Not apt/brew** — workspaced store/shim model.
- **Auto-install on `with`** — may download; not “only if present.”
- **`open` ≠ `with`** — launch vs versioned command execution.
- **Lazy tools** — lint/format may install without an explicit `tool install`.
