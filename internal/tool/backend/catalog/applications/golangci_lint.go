package apps

import "workspaced/internal/tool/backend/catalog"

func init() {
	catalog.RegisterGitHub("golangci-lint", "golangci/golangci-lint")
}
