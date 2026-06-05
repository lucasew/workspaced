package prelude

import (
	_ "workspaced/pkg/checks/formatter/biome"
	_ "workspaced/pkg/checks/formatter/gofmt"
	_ "workspaced/pkg/checks/formatter/prettier"
	_ "workspaced/pkg/checks/formatter/ruff"
	_ "workspaced/pkg/checks/lint/actionlint"
	_ "workspaced/pkg/checks/lint/biome"
	_ "workspaced/pkg/checks/lint/eslint"
	_ "workspaced/pkg/checks/lint/golangci"
	_ "workspaced/pkg/checks/lint/govulncheck"
	_ "workspaced/pkg/checks/lint/ruff"
	_ "workspaced/pkg/checks/lint/shellcheck"
	_ "workspaced/pkg/driver/prelude"
)
