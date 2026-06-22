---
name: workspaced
description: >
  Mental models, subsystems, and gotchas for using workspaced (declarative
  environment / modules / templates / tools / drivers / plan-apply). Reference-
  centric: not a --help mirror, not for implementing workspaced internals.
  Triggers: workspaced, workspaced.cue, modules, templates, tool with, shims,
  home apply, codebase apply, mod lock/tidy, drivers, init, self-install.
---

# Workspaced (usage)

For flags, args, and exact subcommands: run `workspaced [path…] --help`. This
skill explains **systems and surprises**, not argv.

When debugging someone's setup, **read their `workspaced.cue` (and modules)
first**. Init examples and this skill are orientation; their file is the local
dialect. CUE is expressive — see `references/cue.md`.

## Not this skill

- Implementing or extending workspaced itself (drivers, backends, schema in Go):
  see repo `AGENTS.md` / `CODEMAP.md`.

Template authoring for contributors and users lives in this skill:
`references/templates.md` (canonical; replaces the old repo-root `TEMPLATES.md`).

## Load references by topic (usually 1–2 files)

| Question | Open |
|----------|------|
| What is this / how pieces fit? | `references/overview.md` |
| Which config / which tree / cwd? | `references/config-and-roots.md` |
| How to think in CUE here (flexible)? | `references/cue.md` |
| Modules, inputs, lock, tidy/lock | `references/modules.md` |
| How files land in home/repo (templates) | `references/templates.md` |
| Install / run / PATH / `tool with` | `references/tools-and-access.md` |
| Live OS actions (audio, WM, open, …) | `references/drivers.md` |
| Plan vs apply; home vs codebase | `references/plan-and-apply.md` |
| Where are demos / experiments? | `references/demos-and-experiments.md` |

Paths are relative to this skill directory (`skills/workspaced/`).

## Universal (even if you skip other refs)

1. **Cue = intent; lock = pins.** Author and reason in `workspaced.cue` + module
   sources; refresh `workspaced.lock.json` via `mod lock` / `mod tidy` (tidy is
   an alias for lock). Don't treat the lock as primary human config.
2. **Plan before apply** when converging state (`home plan` / `codebase plan`,
   then apply). Apply mutates.
3. **Three different jobs in one binary:** declarative converge (modules +
   apply), obtain binaries (tools), act on the OS now (drivers). Mixing them
   mentally is the usual confusion.
4. **`--help` is live truth for CLI.** This skill will drift less if you don't
   copy flag lists here.

## Shallow command map (discovery only)

| Area | Role |
|------|------|
| `init` / `self-install` | Bootstrap config/modules; install binary into tool system |
| `home` | User/dotfiles plan, apply, sync, backup, config inspect |
| `codebase` | Project plan, apply, lint, format, config |
| `mod` | Refresh lock for enabled modules (`lock` / `tidy`) |
| `tool` | Search, install, list, which, **with** (one-shot versions) |
| `open` | Open files/URLs/webapps/terminal; mise helpers |
| `driver` | Imperative OS capabilities |
| `system` / `svc` / `utils` | Less central; inspect `--help` |
| `experiments` | Demos / experiments — not the daily path |

## Suggested first reads for common goals

- **New / understand the product:** `overview.md` then `modules.md` + `plan-and-apply.md`
- **Change dotfiles / enable something:** user's cue + `modules.md` + `templates.md` + `plan-and-apply.md`
- **Run a tool without committing PATH:** `tools-and-access.md`
- **Clever cue / host-specific / shared defs:** `cue.md` then `modules.md`
- **TUI / taskgroup demos:** `demos-and-experiments.md` only
