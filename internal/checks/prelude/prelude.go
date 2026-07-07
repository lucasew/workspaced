package prelude

import (
	_ "workspaced/internal/checks/formatter/biome"
	_ "workspaced/internal/checks/formatter/gofmt"
	_ "workspaced/internal/checks/formatter/prettier"
	_ "workspaced/internal/checks/formatter/ruff"
	_ "workspaced/internal/checks/lint/actionlint"
	_ "workspaced/internal/checks/lint/biome"
	_ "workspaced/internal/checks/lint/eslint"
	_ "workspaced/internal/checks/lint/golangci"
	_ "workspaced/internal/checks/lint/govulncheck"
	_ "workspaced/internal/checks/lint/ruff"
	_ "workspaced/internal/checks/lint/shellcheck"
)
