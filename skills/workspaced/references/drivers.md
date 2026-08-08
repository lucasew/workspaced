# Drivers

## Model

Drivers are the imperative face of workspaced: do this on the machine now.
Volume, brightness, workspaces/WM, screenshots, wallpaper, notifications,
openers, power, camera, etc. live under `workspaced driver …` (and sometimes
related top-level helpers like `open`).

They are not how you converge dotfiles. That is modules + plan/apply.

Internally, each capability is an interface with one or more implementations;
the program picks one using compatibility checks and weights from configuration.
Users mostly care when something doesn't work on their desktop portal/stack —
then weights/config or environment matter.

## When to use drivers vs apply

| Goal | Use |
|------|-----|
| Make `~/.config/…` / repo files match modules | `home` / `codebase` plan & apply |
| Change volume, take screenshot, move WM focus, set wallpaper now | `driver …` |
| Open a URL/file/webapp | `open` / driver open as applicable |
| Install/run a CLI version | `tool …` / `tool with` |

If the user wants a persistent preference (always this wallpaper path in
config), that may be module/template/cue plus apply — or a one-shot driver call
for today only. Don't assume driver mutations are recorded as declarative state
unless something in modules mirrors them.

## Discovering capabilities

```bash
workspaced driver --help
workspaced driver <category> --help
```

Categories evolve; don't memorize the tree in this skill. `driver doctor` (if
present in your build) can help sanity-check driver/environment issues.

## Config connection

Driver weights and preferences can appear in `workspaced.cue` (evaluated
config). That still isn't "apply installs my bashrc" — it influences which
implementation runs when you invoke a driver command.

Home config load at process start can feed weights early; details are in program
behavior, not required for normal use.

## Relation to other subsystems

| Subsystem | Relation |
|-----------|----------|
| Modules/templates | Declarative files/state; drivers are live actions |
| Tools | Binaries/versioning; drivers may use tools/OS APIs underneath |
| Open | User-facing launch helpers adjacent to opener drivers |
| Experiments/demos | Unrelated showcases; not driver tutorials |

## Gotchas

- Using apply to turn off the screen (or driver to install dotfiles): wrong job;
  see table above.
- Driver worked once, fails on another machine/DE: implementation selection or
  compatibility, not necessarily a cue syntax issue.
- Expecting driver effects in `home plan`: plan/apply track module convergence,
  not every imperative driver call you made earlier.
- Assuming one implementation: Wayland vs X11, portal vs direct, etc. can change
  which backend runs; weights/config/environment matter.
- Scripting drivers without checking help: subcommands differ per category;
  always `--help` on the category you need.
- Confusing `open` with file apply: open launches/views; apply writes declared
  state from modules.
