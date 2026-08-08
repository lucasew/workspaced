package apps

import "workspaced/pkg/tool/backend/catalog"

func init() {
	catalog.RegisterGitHub("golangci-lint", "golangci/golangci-lint")
}
