g Workspaced Development

**Essential reading order for agents**:
1. [README.md](README.md) — one-sentence terms + minimal architecture.
2. [CODEMAP.md](CODEMAP.md) — quick navigation + how to find things.
3. This file — hard rules and gotchas.
4. [TEMPLATES.md](TEMPLATES.md) — **critical** before touching any templates.

## Hard Rules (break these and things will be wrong)

- **Config is CUE-first only**. Never fall back to `GlobalConfig.Merge()` patterns.
- **Adding config fields** (always in this order):
  1. Update schema + defaults in `pkg/configcue/schema.cue` (and preambles if needed).
  2. Add the key in the user's `workspaced.cue`.
  3. Decode with `configcue.Config.Decode()` or `ModuleConfig()`.
  4. Consume via `{{ .Field }}` in templates.
- **No lists** in module configs. Only deep-merge friendly shapes.
- **Driver preloads**: import `_ "workspaced/pkg/driver/prelude"` **only** from `cmd/workspaced/root.go`. Never duplicate in subcommands.
- **Process execution**: always use `pkg/driver/exec` outside of driver implementations.
- **Network**: use the `fetchurl` driver when you have a hash; otherwise the `httpclient` driver. Never touch `http.DefaultClient` directly.
- **Tool backends**: scope on backends (github, mise, catalog), never on individual tools. Prefer the word "backend".
- **Zero intermediate files**: module processing must stream in-memory.
- **Avoid having potentially stale information in Markdown files**: code is king, docs is consequence, prefer having references to code on docs instead of duplicating stuff around.

## Driver System (minimal)

Drivers implement `DriverFactory[T]`:
- `ID()`, `Name()`, `CheckCompatibility()`, `New()`

Selection is done by `driver.Get[T](ctx)` using weights from `workspaced.cue`.

Register by importing the impl package from the central prelude.

## When Adding Things

- **New driver category**: create `pkg/driver/<cat>/driver.go` (interface) + `facade.go`, one impl dir, add import to `pkg/driver/prelude/prelude.go`, and CLI bits under `cmd/workspaced/driver/<cat>/`.
- **New tool backend**: add under `pkg/tool/backend/<name>/`. For curated short names, put in `catalog/applications/`.
- **New module source**: implement in `pkg/modfile/sourceprovider/`.

## Module Lock Model

- `workspaced.cue` = declarative inputs.
- `workspaced.lock.json` = resolved pins (source URLs, hashes, tool versions).

## Anti-Patterns

- Duplicating prelude imports.
- Using raw `os/exec` or `http.DefaultClient` in feature code.
- Treating tools as first-class backends instead of going through registries/backends.
- Putting lists in module config.

See CODEMAP.md for the short "how to locate X" recipes.

## Patterns
- The context argument is **always** ctx
- `logger = logging.GetLogger(ctx)`. Never use log/slog directly.
- Make sure a inner scope is not getting a logger or ctx from the outer scope.
- Prefer to use channels  over shared mutable state with a lock if it makes stuff simpler, which is often.
- All context.Background and alike situations must have a good reason. Just not having a context in scope is not a good reason.
- Actual data that could be piped to another program should be written to stdout. The rest of command output must be written to stderr. stdout is normally only used to pipe data suitable to other command line software where it must not have to take more than one line into consideration for a specific operation (things where each thing is a line, or JSONL).
