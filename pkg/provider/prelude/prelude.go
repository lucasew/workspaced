package prelude

import (
	_ "workspaced/pkg/provider/formatter/biome"
	_ "workspaced/pkg/provider/formatter/gofmt"
	_ "workspaced/pkg/provider/lint/biome"
	_ "workspaced/pkg/provider/lint/golangci"
)
