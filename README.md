# WorkspaceD

- Modular templates for dotfiles, plus drivers that abstract common system tools.
- Most of what people want from Nix/NixOS, without the slow path.

- Library import path is `pkg/` (`api`, `driver`, `logging`, `palette`, `taskgroup`); the rest is `internal/` + `cmd/`. Details in AGENTS.md.

## Terminology

- Driver: interface over OS services (audio, workspaces, cameras, …). Impls register via `DriverFactory`.
- Linter: reports problems in a codebase or system.
- Formatter: rewrites code into a fixed shape without changing behavior.
- Tool: lists versions and installs a scoped program for one version. A version usually ships assets and at least one binary.
- Tool backend: turns a ref into a Tool. Example: the GitHub backend takes `owner/repo`.
- Module: a slice of CUE config defined elsewhere, plus the file templates that go with it.
- CUE: JSON-shaped language that unifies toward the most specific type; validation and data live together.
