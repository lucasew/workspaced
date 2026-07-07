package apps

import "workspaced/internal/tool/backend/catalog"

func init() {
	catalog.RegisterGitHub("ruff", "astral-sh/ruff")
}
