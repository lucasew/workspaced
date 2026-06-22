# Tools and access

## Model

A **tool** is a versioned program (usually at least one executable) obtained
through a **backend** (curated registry/catalog, `github:…`, `mise:…`, etc.).
Workspaced can install tools into its store, expose them via **shims**, run
other commands with temporary tool versions (**`tool with`**), and internally
**lazy-ensure** tools when its own features need them (linters, etc.).

Tools are **not** modules. Modules converge files/state; tools provide binaries.

Backends are the abstraction — think “where does this ref resolve?” not “is each
binary a special snowflake in the CLI.”

## Access modes (the important table)

| Mode | What it does | Persistence | Typical use |
|------|--------------|-------------|-------------|
| **`workspaced tool with <specs>… -- <cmd>…`** | Ensure listed tool versions; run **one** command with them available / preferred for resolution | **That invocation** (does not by itself fix your next login shell) | One-shot builds, pin go/node for a command, CI-like runs |
| **`tool install` / install via normal tool flows** | Put tool into workspaced’s tool store | On disk under the tools dir | You want it available going forward via workspaced |
| **Shims** | Thin executables on PATH that delegate into workspaced’s tool resolution | Until you remove shims / PATH | Day-to-day `rg`, `uv`, etc. if shell is set up |
| **Lazy / internal ensure** | Other workspaced commands pull a tool when needed | As installed by that path | Lint/format/codebase features |
| **`open` / mise helpers** | Launchers and mise-oriented entry points | Depends on subcommand | Opening apps, mise-backed workflows |

If the user says “I ran tool with and my terminal still doesn’t have it,” that’s
expected: **`with` is not a substitute for shims + shell init.**

## Layout on disk (orientation)

Under the user data area (conceptually `~/.local/share/workspaced/`):

| Path role | Purpose |
|-----------|---------|
| **`tools/`** | Installed tool versions / store |
| **`shims/`** | Shim executables meant to live on PATH |

Exact paths come from the program (`GetToolsDir` / `GetShimsDir` style layout).
Shell integration often comes from **`workspaced utils shell init`** (or your
dotfiles modules generating equivalent) so shims and env are on PATH — without
that, install/shim can exist on disk and still feel “not installed” in a bare
shell.

## `tool with` (semantics worth knowing)

Documented behavior in the command itself is the flag/arg authority; here is
the **model**:

- Put **tool specs before `--`**, the **command after `--`**. Prefer always
  using `--` (there is legacy single-tool behavior without it; don’t rely on it).
- Specs can look like:
  - `provider:package@version`
  - `provider:package` (latest-ish for that path)
  - `package@version` / `package` — bare/curated names often go through the
    **registry/catalog** (e.g. `ripgrep`, `uv`)
  - `mise:…` / `github:…` when you mean those backends explicitly
- Listed tools are **ensured** (installed if missing) — expect network/side
  effects.
- Multiple tools can be listed; the command binary is resolved using tools that
  actually provide that name (don’t assume a simplistic “only last tool matters
  for everything”).
- Good for **ephemeral** versioned execution; not the only way to access tools.

Examples (illustrative; confirm with `--help`):

```bash
workspaced tool with ripgrep -- rg pattern
workspaced tool with mise:go@1.21.0 -- go version
workspaced tool with github:denoland/deno@1.40.0 -- deno run app.ts
workspaced tool with mise:go@1.21.0 mise:node@20 -- node --version
```

## Other tool verbs (discovery)

| Verb | Role |
|------|------|
| `tool search` | Find tools/refs |
| `tool list` | What’s known/installed in workspaced’s view |
| `tool install` | Install by spec |
| `tool which` | Where a binary resolves |
| `tool versions` / `latest` / `artifacts` | Version/artifact inspection |
| `tool with` | Run with ensured versions |

Always `--help` for flags.

## Backends (user-level)

| Kind | Spec flavor | When |
|------|-------------|------|
| **Registry / catalog** | Short names (`ripgrep`, `uv`, …) | Curated; default for many bare names |
| **GitHub** | `github:owner/repo` | Direct from releases/assets as implemented |
| **Mise** | `mise:tool` | Languages/toolchains mise manages |

If a bare name fails, try an explicit `mise:` or `github:` form, or `tool search`.

## Relation to lock / cue

Some environments pin tools through config/lock workflows (module lock entries
can include tool-oriented pins for renovate/automation). That’s still **pins**,
not a replacement for understanding access modes above. Changing pins → refresh
lock when that’s part of your workflow (`modules.md`).

## Gotchas

- **`tool with` ≠ permanent PATH** — next shell won’t magically see those
  versions unless shims/install/shell init already provide them.
- **Missing `--`** — ambiguous parsing / legacy mode; always separate specs from
  command with `--`.
- **Bare name vs wrong backend** — `go`/`node` often want `mise:`; curated short
  names want registry; random `owner/repo` wants `github:`.
- **Shims not on PATH** — install succeeded, interactive shell still uses system
  or nothing; fix shell init / shim dir on PATH (`utils shell init` / dotfiles).
- **Expected system package manager behavior** — workspaced tools live in its
  store/shims model, not necessarily `/usr/bin` via apt/brew.
- **Auto-install on `with`** — not dry; may download. Fine for agents if
  intended; surprising if the user wanted “only if present.”
- **Confusing `open` with `tool with`** — `open` launches configured/openers;
  `with` is about tool versions on a command line.
- **Lazy internal tools** — codebase lint/format may pull tools you didn’t
  explicitly install; that’s separate from the user’s daily PATH.
