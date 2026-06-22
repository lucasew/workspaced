# Config and roots

## Model

Workspaced loads **CUE configuration** from different places depending on
*which command family* you run and *where you are*. “I edited cue and nothing
happened” is very often **wrong file / wrong root**, not invalid CUE.

There is usually a top-level `workspaced: { … }` (or equivalent evaluated
shape). Inside: hosts, modules, driver weights, inputs, and anything else the
schema/preambles define. Authoring *style* is open; see `cue.md`.

Inspect commands exist under config subtrees, e.g. `home config …` and
`codebase config …` (`get`, `eval`, `dump`, `layers`, … — use `--help`).

## Typical roots (conceptual)

Exact search order is implemented in the loader; treat this as the mental map:

| Context | Rough idea |
|---------|------------|
| **Home / dotfiles** | Config associated with the user environment and `$DOTFILES` / `~/.dotfiles`-style trees; home commands use this world. |
| **Home directory** | A `workspaced.cue` directly under the user home may participate in home loading. |
| **XDG / config dir** | Another possible home-config location depending on setup. |
| **Codebase** | Walk **up** from the current (or project) directory for the **nearest** `workspaced.cue`. |

Codebase / walk-up behavior matters a lot:

- Nested **git** repos generally **do not** inherit an outer repo’s
  `workspaced.cue` when searching upward across that boundary. A project inside
  another project may have its own cue — or none.
- Closest cue wins in walk-up scenarios; parent cues are not magically merged
  across that search just because they exist higher up.

So: always know **which tree** `home` vs `codebase` is operating on, and which
`workspaced.cue` path that implies.

## Home vs codebase (config angle)

| | **Home** | **Codebase** |
|---|----------|----------------|
| **Intent** | User/dotfiles environment | This repository / project |
| **Typical ops** | plan, apply, sync, backup | plan, apply, lint, format |
| **Cue** | Home/dotfiles/config-dir family | Nearest project `workspaced.cue` |

Same binary, different loaders and goals. See also `plan-and-apply.md`.

## Lockfile placement

`workspaced.lock.json` sits next to the workspace / config it belongs to (module
resolution and pins for that environment). Changing cue inputs/modules without
refreshing lock in the **same** workspace is a classic source of “stale pins” or
surprising apply.

`mod lock` / `mod tidy` detect a workspace and rewrite that lock; they are not
global for every cue on disk.

## What to read when debugging

1. The command you ran (`home` vs `codebase` vs `mod`).
2. Cwd and whether you’re inside a nested git repo.
3. The actual `workspaced.cue` path that should apply (open it; don’t assume
   `~/.dotfiles/workspaced.cue` if the user lives elsewhere).
4. Nearby `workspaced.lock.json` if modules/tools pins are involved.

Use config inspect subcommands when you need evaluated/layered views rather than
raw file read alone.

## Gotchas

- **Valid CUE, ignored by workspaced** — wrong root key, wrong file, or command
  family that never loads that path. Evaluation succeeding in abstract CUE tools
  ≠ workspaced loaded it.
- **Edited parent cue, ran command in child git repo** — child may not see parent
  cue; you “fixed” a file that this invocation never reads.
- **Home apply vs codebase apply** — same word “apply”, different targets; home
  won’t fix a project lint config in the repo, and codebase won’t converge
  `~/.config` from dotfiles modules.
- **Lock next to another workspace** — you refreshed lock in tree A, apply in
  tree B; pins and intent diverge.
- **`$DOTFILES` / init defaults** — init scaffolds under conventional paths;
  long-lived users often diverge. Trust paths on disk over docs defaults.
