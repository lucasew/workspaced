package prelude

import (
	_ "workspaced/pkg/driver/prelude"
	_ "workspaced/pkg/provider/formatter/biome"
	_ "workspaced/pkg/provider/formatter/gofmt"
	_ "workspaced/pkg/provider/formatter/ruff"
	_ "workspaced/pkg/provider/lint/biome"
	_ "workspaced/pkg/provider/lint/golangci"
	_ "workspaced/pkg/provider/lint/ruff"
	_ "workspaced/pkg/tool/prelude"
)
