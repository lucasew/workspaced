# Overview

## What workspaced is

A **declarative user-environment manager**: CUE config selects and parameterizes
**modules** (bundles of files/templates/settings), resolves **inputs** (where
module sources come from), pins versions in a **lockfile**, and **applies** the
desired tree into home (dotfiles) or a codebase.

Alongside that, the same binary exposes:

- **Tools** — versioned executables (github / mise / curated registry backends),
  installable, lockable, or runnable one-shot.
- **Drivers** — imperative OS actions (audio, brightness, WM, screenshot, open,
  …), selected by compatibility + weights in config.

Roughly: NixOS/home-manager *ideas* (declarative modules, pins, apply), aimed at
being faster and more direct on normal Linux/macOS/Android setups.

## How pieces connect

```text
workspaced.cue  ──►  modules + inputs (+ other config)
        │                    │
        │                    ├──► workspaced.lock.json  (resolved pins)
        │                    │
        │                    └──► plan / apply  ──► files & converged state
        │
        ├── tool refs / backends  ──► install | shims | `tool with`
        └── driver weights        ──► `driver …` (live OS)
```

Templates live *inside modules* (and related source layout). Apply is what
materializes them; editing a template alone only changes source.

## Mental model: three jobs

| Job | Typical commands | Nature |
|-----|------------------|--------|
| **Converge environment** | `home` / `codebase` plan & apply, `mod lock` | Declarative; cue + modules + lock |
| **Get a binary** | `tool …`, `tool with … -- …` | Versioned tools; store + shims + one-shot |
| **Do something on the OS now** | `driver …`, some `open …` | Imperative; not “apply my dotfiles” |

Agents and humans get unstuck faster when they classify the task into one row
before reaching for flags.

## Bootstrap (orientation only)

Typical first-time arc (details via `--help` and your tree):

1. `self-install` — place the binary in the tool/shim system.
2. `init` — starter `workspaced.cue`, example module under dotfiles, host/IP hints.
3. Edit cue / modules in *your* style (CUE is flexible; see `cue.md`).
4. `mod lock` (or `mod tidy`) when inputs/modules need lock refresh.
5. `home plan` then `home apply` when you want real writes.

Init is a **starter**, not the only valid cue shape.

## Config surfaces (names only)

- **`workspaced.cue`** — primary human/agent intent.
- **`workspaced.lock.json`** — resolved pins (sources, hashes, tool versions);
  useful for reproducibility and automation (e.g. Renovate-oriented fields);
  not the place to “design” the environment.
- **Module trees** — e.g. `modules/<name>/` under dotfiles for `input: "self"`.

Which physical `workspaced.cue` loads depends on home vs codebase vs walk-up;
see `config-and-roots.md`.

## Related docs in the workspaced *source* repo

| Doc | Audience |
|-----|----------|
| `README.md` | One-line terms |
| `AGENTS.md` / `CODEMAP.md` | Contributing to workspaced itself |
| This skill (other `references/`) | Modules, tools, drivers, cue, plan/apply |

Templates are fully documented in this skill’s `references/templates.md` (canonical).

This skill is for **operating** workspaced, including in other repos that only
consume it.
