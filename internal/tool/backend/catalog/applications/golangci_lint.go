package apps

import "github.com/lucasew/workspaced/internal/tool/backend/catalog"

func init() {
	catalog.RegisterGitHub("golangci-lint", "golangci/golangci-lint")
}
