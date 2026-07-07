package apps

import "workspaced/internal/tool/backend/catalog"

func init() {
	catalog.RegisterGitHub("shfmt", "patrickvane/shfmt")
}
