package registry

import (
	"workspaced/pkg/tool/provider"
	githubprov "workspaced/pkg/tool/provider/github"
)

// namedTools defines the curated set of "registry applications" (named tools)
// that the registry provider knows about.
//
// These are short, memorable names that users can reference directly
// (e.g. "registry:uv" or just "uv" when registry is the default provider).
//
// Each entry maps to a concrete Tool implementation by calling the
// exposed constructor from the appropriate backend provider (currently only
// github-backed tools are registered here).
//
// The dispatch logic lives in Provider.Tool (inlined for simplicity).
// A package-level map is used here for readability and easy maintenance.
// A large switch statement could replace it if zero-allocation dispatch
// becomes important.
//
// Only github-backed tools are included. For mise tools (or anything else),
// users must use the explicit "mise:" or "github:" provider prefixes.
var namedTools = map[string]func() (provider.Tool, error){
	// Curated short names that map to GitHub releases.
	"uv":            func() (provider.Tool, error) { return githubprov.NewTool("astral-sh/uv") },
	"ruff":          func() (provider.Tool, error) { return githubprov.NewTool("astral-sh/ruff") },
	"fzf":           func() (provider.Tool, error) { return githubprov.NewTool("junegunn/fzf") },
	"ripgrep":       func() (provider.Tool, error) { return githubprov.NewTool("burntsushi/ripgrep") },
	"rg":            func() (provider.Tool, error) { return githubprov.NewTool("burntsushi/ripgrep") },
	"fd":            func() (provider.Tool, error) { return githubprov.NewTool("sharkdp/fd") },
	"golangci-lint": func() (provider.Tool, error) { return githubprov.NewTool("golangci/golangci-lint") },
	"shellcheck":    func() (provider.Tool, error) { return githubprov.NewTool("koalaman/shellcheck") },
	"actionlint":    func() (provider.Tool, error) { return githubprov.NewTool("rhysd/actionlint") },
	"biome":         func() (provider.Tool, error) { return githubprov.NewTool("biomejs/biome") },
	"shfmt":         func() (provider.Tool, error) { return githubprov.NewTool("patrickvane/shfmt") },
	"sops":           func() (provider.Tool, error) { return githubprov.NewTool("getsops/sops") },
	"docker-compose": func() (provider.Tool, error) { return githubprov.NewTool("docker/compose") },
	"terraform":     func() (provider.Tool, error) { return githubprov.NewTool("hashicorp/terraform") },
	"tflint":        func() (provider.Tool, error) { return githubprov.NewTool("terraform-linters/tflint") },

	// Additional names from user's workspaced.cue (github-backed)
	"opencode": func() (provider.Tool, error) { return githubprov.NewTool("anomalyco/opencode") },
	"rclone":   func() (provider.Tool, error) { return githubprov.NewTool("rclone/rclone") },
	"rtk":      func() (provider.Tool, error) { return githubprov.NewTool("rtk-ai/rtk") },

	// Add more curated github names here as needed.
}
