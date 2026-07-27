// Package prelude registers the standard workspaced tool backends so
// github.com/lucasew/workspaced/pkg/tool.EnsureInstalled can resolve specs
// such as registry:mise without the CLI process.
//
// Blank-import this package from external programs (not from workspaced's own
// pkg/drivers — those stay on internal/tool/prelude via cmd/).
package prelude

import (
	_ "github.com/lucasew/workspaced/internal/tool/backend/catalog"
	_ "github.com/lucasew/workspaced/internal/tool/backend/catalog/applications"
	_ "github.com/lucasew/workspaced/internal/tool/backend/github"
	_ "github.com/lucasew/workspaced/internal/tool/backend/mise"
)
