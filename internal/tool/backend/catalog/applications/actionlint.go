package apps

import "github.com/lucasew/workspaced/internal/tool/backend/catalog"

func init() {
	catalog.RegisterGitHub("actionlint", "rhysd/actionlint")
}
