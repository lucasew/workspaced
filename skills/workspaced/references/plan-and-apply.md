# Plan and apply

Whole file is essential and short. Invocation/flags: `workspaced home plan|apply
--help` (and codebase equivalents). Universals: SKILL.md.

## Model

Plan computes desired state from cue + modules (+ lock/resolution as needed)
and compares to reality, reporting what would change. Prefer this whenever an
agent or human is about to mutate an environment.

Apply executes convergence: writes, links, removes, or other declared actions so
reality matches the desired module output.

Same words appear under different roots:

| Family | Typical intent |
|--------|----------------|
| `home plan` / `home apply` | User/dotfiles environment |
| `codebase plan` / `codebase apply` | Current project / nearest codebase cue |
| `system apply` (and friends) | System-oriented flows (separate; inspect `--help`) |

Config loading differs by family (`config-and-roots.md`). Planning home never
answers "what would codebase lint apply do?"

## Recommended loop

1. Edit cue and/or module sources (templates/static files).
2. If inputs/modules/versions changed: `mod lock` / `mod tidy` in the right
   workspace.
3. `… plan` — read the actions; adjust cue/modules if surprising.
4. `… apply` — only when writes are intended.
5. Re-plan if something still looks wrong.

Dry-run / verbose global flags may exist on the root command (`--help` on
`workspaced`); use them when you want more signal without guessing.

## Home extras (not the same as apply)

Under `home` you may also find:

| Area | Role |
|------|------|
| sync | Synchronization workflows (not identical to apply) |
| backup | Backup actions |
| config | Inspect/evaluate home config |

Don't substitute backup/sync for "I changed a template and want it rendered"
unless that really is the user's goal.

## Codebase extras

| Area | Role |
|------|------|
| lint / format | Run checks/formatters (may pull tools lazily) |
| config | Inspect codebase workspaced config |
| ci-status etc. | Ancillary project helpers |

Lint/format are operational on the repo; they may use the tool subsystem
internally. Still not drivers.

## What plan/apply see

Roughly: enabled modules, their resolved sources, rendered/static outputs, and
deployer/planner logic for create/update/delete-style actions. They do not undo
arbitrary driver clicks or explain every tool shim on PATH.

If plan is empty but you expected changes:

- Wrong command family (home vs codebase)
- Wrong cwd / cue root
- Module not enabled or wrong path
- Lock/source resolution not updated
- Template `skip`/conditionals suppressing output
- Change only in an untracked file outside module layout

## Gotchas

- Apply without plan: agents especially should avoid; surprises include deletes
  and overwrites you didn't narrate.
- Plan, then edit cue, then apply without re-plan: apply runs current desired
  state, not the plan output you saw earlier.
- Home vs codebase mixup: "apply didn't update the repo" because you ran
  `home apply`, or vice versa.
- Lock/module drift: plan/apply against stale pins after cue input changes;
  lock first (`modules.md`).
- Expecting plan to list driver/`tool with` effects: different subsystems.
- Global dry-run flag vs plan: plan is the first-class "what would change" for
  module convergence; still check root `--help` for additional modes.
- Assuming apply is always safe: it's the mutation step; permissions,
  overwrites, and removals depend on planner/executor and module design.
