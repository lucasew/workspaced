package apps

import "github.com/lucasew/workspaced/internal/tool/backend/catalog"

func init() {
	catalog.RegisterGitHub("ruff", "astral-sh/ruff")
}
