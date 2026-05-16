## IGNORE: Unscoped File Deletions & Mass Refactoring

**- Pattern:** Deleting files, commands, or documentation sections (like `AGENTS.md` or `TEMPLATES.md`) that are completely unrelated to the explicit task (e.g., while adding a SARIF upload feature).
**- Justification:** Expanding scope to delete unrelated code or documentation introduces massive risk and violates the core directive of scope discipline.
**- Files Affected:** `*` (Any file not directly related to the requested feature)

## IGNORE: Downgrading CI/CD Actions and Dependencies

**- Pattern:** Downgrading GitHub Action versions (e.g., `actions/checkout` to v4 from v5, or `peter-evans/create-pull-request` to v6 from v8).
**- Justification:** Reverting dependencies to older major versions introduces regressions, undoes security/pinning updates, and violates global instructions not to downgrade unless explicitly requested.
**- Files Affected:** `.github/workflows/*.yml`

## IGNORE: Hallucinated Dependency Versions

**- Pattern:** Bumping tool versions to non-existent or hallucinated future versions (e.g., Node.js v25.6.1, golangci-lint v2.10.1).
**- Justification:** Automated updates to versions that don't actually exist break the build environment and demonstrate a lack of real-world validation.
**- Files Affected:** `mise.toml`, `go.mod`

## IGNORE: Unhandled Errors in Go Code

**- Pattern:** Ignoring error returns from standard I/O or formatting functions without explicitly checking them (e.g., `fmt.Fprintln`, `fmt.Fprintf`, `resp.Body.Close()`).
**- Justification:** Project conventions mandate that Go code must explicitly handle all errors. Introducing new unhandled errors will fail the strict linter checks and violate core guidelines.
**- Files Affected:** `*.go`

## IGNORE: Improper CLI Output Formatting (Replacing Prints with slog)

**- Pattern:** Replacing user-facing CLI instructions or generic interactive prints with structured logging (e.g., swapping `fmt.Printf` with `slog.Info` for user messages).
**- Justification:** Structured logging (`slog`) is strictly for machine-readable application logs. Standard human-facing instructions and prompts must use `stderr` (e.g., `fmt.Fprint(os.Stderr, ...)`). Conflating the two breaks CLI UX.
**- Files Affected:** `cmd/**/*.go`
