# Spec: CUE-driven checks + `lint --review`

Status: accepted (grill 2026-07-19). One-shot implementation.

## Goals

1. Declare **linters and formatters** in CUE (prelude defaults + workspace overrides), same spirit as `lsp`.
2. Drop per-tool Go packages; keep a **generic runner** and a **closed codec set** for non-SARIF tools.
3. `workspaced codebase lint --review` posts **GitHub Actions workflow-command annotations** for findings on the **relevant diff** (not PR review comments).

## Config shape

```
workspaced: {
  lint: { tools: [string]: #CheckTool }
  formatter: { tools: [string]: #CheckTool }
}
```

### `#CheckTool`

| Field | Lint | Format | Notes |
|-------|------|--------|-------|
| `enable` | yes | yes | default true |
| `detect` | yes | yes | ordered map of firewall rules |
| `needs` | yes | yes | `lazy_tools` names → ensure before run |
| `cmd` | yes | yes | argv; format cmds include write flags |
| `output` | **required** | absent | codec name |
| `args_from_globs` | optional | optional | append files matching winning rule's glob |

### Detect rules (firewall)

Keys sort lexicographically (`00-…` first). **First matching rule wins**; its `enable` decides applicability.

Per rule:

- `path` — match if file or directory exists under the run root
- `glob` — match if any file under root matches (supports `**` and simple `{a,b}` braces)
- `enable` — required bool
- At least one of `path` / `glob` must be set for a rule to match

No match → tool skipped. Empty `detect` → skipped. **No bin checks.**

## Runner

1. Load codebase config for the path/git root.
2. For each enabled tool, evaluate detect; skip if not applicable.
3. Resolve `needs` via lazy_tools (PATH + absolute cmd[0] when names match).
4. If `args_from_globs`, expand winning rule glob and append relative paths to argv.
5. Run with `Dir = root`.
   - **Lint:** capture stdout; decode via codec → SARIF run. Non-zero exit with empty stdout is failure; with findings stdout, treat as success for tool (codec may still fail).
   - **Format:** stdout/stderr attached (passthrough); non-zero is failure. Serial execution.
6. Bundle SARIF runs; omit tools that hard-fail (log); same as today for lint.

### Codecs (`output`)

| Name | Meaning |
|------|---------|
| `sarif` | tool prints SARIF; take first run |
| `actionlint_json` | actionlint `-format '{{json .}}'` |
| `shellcheck_json` | shellcheck `-f json` |
| `eslint_json` | eslint `-f json` (with existing sanitize helpers) |

New tools should prefer `sarif`.

## `lint --review`

| Aspect | Behavior |
|--------|----------|
| Flag | `--review` on `codebase lint` |
| When active | `GITHUB_ACTIONS` is true/1 |
| Otherwise | warn + soft no-op (linters still run) |
| Mechanism | workflow commands `::error` / `::warning` / `::notice` on stdout |
| Filter | only findings on **relevant diff** lines |
| Diff | (1) base…HEAD when base known (`GITHUB_BASE_REF` / event base SHA / env); (2) else last commit `HEAD~1…HEAD` |
| No usable diff | warn + no annotations |
| Exit code | **unchanged by findings**; non-zero only if a linter **run fails** (setup/exec/parse) |

Not in v1: Checks API named runs, PR review comments, fail-on-findings, local `gh pr` posting.

## Defaults (prelude)

Migrate current built-ins:

**Lint:** golangci-lint, govulncheck, ruff, biome, actionlint, shellcheck, eslint  
**Format:** gofmt, ruff, biome, prettier  

`lazy_tools` entries remain in prelude for pins.

## Layout

- Schema/prelude: `internal/configcue/`
- Runner + detect: `internal/checks/`
- Codecs: `internal/checks/codec/`
- Review annotations: `internal/checks/review/`
- CLI: `cmd/workspaced/codebase/lint.go`, `format.go`
- Remove: `internal/checks/lint/<tool>/`, `formatter/<tool>/`, blank prelude imports

## Success criteria

- `go test ./internal/checks/...` passes (detect, codecs, review filter, config decode)
- `go build ./cmd/workspaced` succeeds
- With prelude defaults, `codebase lint` / `format` still apply tools when detect matches
- `--review` off GHA: warning, no commands; on GHA with diff: only on-diff lines annotated
