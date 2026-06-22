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

When debugging someone's setup, **read their `workspaced.cue` (and modules)
first**. Init and this skill are orientation; their file is the local dialect.

## Sources of truth (anti-duplication)

Keep **one home** per class of fact. Agents: do not paste the same fact into
two references; **link** or say “see X”.

| Fact class | Canonical place | Not here |
|------------|-----------------|----------|
| Flags, arg shapes, invocation examples | **`workspaced … --help`** (and `Example`/`Long` on the command) | Skill body, tables of every subcommand |
| Mental model, when/why, cross-subsystem gotchas | **This skill** (`SKILL.md` + `references/`) | Duplicating help Examples blocks |
| Template kinds, functions, pitfalls | **`references/templates.md` only** | Second template guide elsewhere in repo |
| CUE **language** (unify, lattice, defs, `?`/`!`, defaults) | **`skills/cue-language/`** | Re-teaching CUE in this skill’s `cue.md` |
| Workspaced **host** contracts for CUE (roots, evaluated keys, merge edges) | **`references/cue.md`** + `modules.md` | cue-language skill (it points here instead) |
| Hard contributor rules (CUE schema order, preludes) | **`AGENTS.md` / `CODEMAP.md`** | This skill (usage only) |
| User’s real config | **Their `workspaced.cue` + modules on disk** | Assuming init shape is universal |

**Skill owns:** *what the subsystem is*, *how it connects*, *surprises agents hit
even with correct flags*.

**CLI help owns:** *how to invoke this exact command today*.

If help is thin on a command, improve **that command’s `Long`/`Example` in Go**
rather than growing the skill. High-value help targets (when editing code):
`tool with`, `home apply`/`plan`, `mod lock`, `init` — role in one paragraph +
2–4 examples; skill should only add *ephemeral vs shims*, *plan vs apply*, etc.

## Essential vs deep (how much to load)

**Always (this file only):** universals below. Usually enough for triage.

**Essential references** — load when the task touches that subsystem; read
**Model** + **Gotchas** (and any section titled **Essential**); stop unless
authoring.

**Deep** — only when actively authoring/debugging that area (full functions
lists, generator fast-path, inspect internals, every practical example).

| Tier | Question | Open | Default depth |
|------|----------|------|----------------|
| Essential | What is this / how pieces fit? | `references/overview.md` | Whole file is short |
| Essential | Wrong config / wrong tree? | `references/config-and-roots.md` | Whole file |
| Essential | Plan vs apply; home vs codebase | `references/plan-and-apply.md` | Whole file |
| Essential | Modules / inputs / lock | `references/modules.md` | Model + lock + gotchas |
| Essential | Tools / PATH / `tool with` meaning | `references/tools-and-access.md` | Access modes + gotchas; **not** help examples |
| Essential | Drivers vs apply | `references/drivers.md` | Whole file is short |
| Deep | Workspaced + CUE host contracts | `references/cue.md` | When writing/refactoring *workspaced* cue |
| Deep | CUE language mechanics | **`../cue-language/`** (sibling skill) | Unify/`_|_`/defs/disjunctions/defaults — not duplicated here |
| Deep | Writing/editing templates | `references/templates.md` | Kinds table first; functions/examples/internal/generator only if authoring |
| Optional | Demos / experiments | `references/demos-and-experiments.md` | Only if exploring `experiments`/`utils demo` |

Paths are relative to `skills/workspaced/` unless noted. **Usually 1 essential
ref**, plus deep only if editing that subsystem.

## With `cue-language` (sibling skill)

Both may trigger on `workspaced.cue`. **Order matters:**

1. **Product / ops first** (this skill): wrong root, modules, lock, plan/apply,
   tools, drivers, templates — even if the file extension is `.cue`.
2. **Language when the obstacle is CUE** (`skills/cue-language/`): unification
   fails, `_|_`, defaults (`*`), definitions (`#…`), optional/required (`?`/`!`),
   closedness, comprehensions as *language*.
3. **Both when authoring non-trivial workspaced config:** cue-language (legal
   CUE) then this skill’s `references/cue.md` + `modules.md` (usable for this
   host). Always prefer the user’s on-disk cue over either skill’s examples.

Do **not** load cue-language for apply/lock/tools/drivers-only tasks. Do **not**
use cue-language answers as a substitute for which `workspaced.cue` loads or
whether to plan/apply.

## Not this skill

- Implementing workspaced internals: `AGENTS.md` / `CODEMAP.md`.
- Argv encyclopedias: `workspaced [path…] --help`.
- CUE language textbook: `skills/cue-language/` (see above).

## Universals (only facts allowed to repeat everywhere in one line)

Elsewhere in references, **link “see SKILL universals”** instead of restating
paragraphs.

1. **Cue = intent; lock = pins** — edit cue/modules; `mod lock` / `mod tidy`
   (tidy ≡ lock) refreshes lock; lock is not primary human config.
2. **Plan before apply** — `home`/`codebase` plan then apply; apply mutates.
3. **Three jobs** — converge (modules+apply) ≠ binaries (tools) ≠ live OS
   (drivers).
4. **CLI help is live** — skill does not mirror flags/examples.

## Command map (names only — help for details)

| Area | Role in one breath |
|------|-------------------|
| `init` / `self-install` | Bootstrap |
| `home` / `codebase` | Plan/apply (+ home sync/backup; codebase lint/format) |
| `mod` | Lock refresh |
| `tool` | Install/search/`with` |
| `open` / `driver` | Launch / imperative OS |
| `experiments` / some `utils demo` | Not daily path (`demos-and-experiments.md`) |

## Goal → read (essential first)

- **Orient:** `overview.md` → `modules.md` (essential) → `plan-and-apply.md`
- **Change dotfiles:** user’s cue + `modules.md` essential; if editing files
  under `config/`, then `templates.md` (deep as needed) + plan/apply
- **One-shot tool:** `tools-and-access.md` essential; `tool with --help` for specs
- **Clever / non-trivial cue for workspaced:** `skills/cue-language/` for
  mechanics, then `references/cue.md` + `modules.md` for host contracts
- **CUE unify/`_|_`/defs only (not workspaced behavior):** `skills/cue-language/`
  alone; add this skill only if apply/lock/roots/modules are in play
- **Demos only:** `demos-and-experiments.md`
