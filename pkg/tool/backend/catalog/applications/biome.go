package apps

import "workspaced/pkg/tool/backend/catalog"

func init() {
	catalog.RegisterGitHub("biome", "biomejs/biome")
}
