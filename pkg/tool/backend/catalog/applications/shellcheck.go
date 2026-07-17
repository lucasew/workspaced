package apps

import "workspaced/pkg/tool/backend/catalog"

func init() {
	catalog.RegisterGitHub("shellcheck", "koalaman/shellcheck")
}
