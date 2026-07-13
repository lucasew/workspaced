package apps

import "workspaced/internal/tool/backend/catalog"

func init() {
	// Official shfmt lives under mvdan/sh (patrickvane/shfmt is a stale fork
	// whose release assets lack linux/arm64).
	catalog.RegisterGitHub("shfmt", "mvdan/sh")
}
