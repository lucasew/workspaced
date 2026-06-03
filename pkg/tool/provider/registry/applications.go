package registry

import (
	"workspaced/pkg/tool/provider"
	githubprov "workspaced/pkg/tool/provider/github"
)

func WrapNewTool(f func(ref string) (provider.Tool, error), ref string) func() (provider.Tool, error) {
	return func() (provider.Tool, error) {
		return f(ref)
	}
}

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
	"uv":             WrapNewTool(githubprov.NewTool, "astral-sh/uv"),
	"ruff":           WrapNewTool(githubprov.NewTool, "astral-sh/ruff"),
	"fzf":            WrapNewTool(githubprov.NewTool, "junegunn/fzf"),
	"ripgrep":        WrapNewTool(githubprov.NewTool, "burntsushi/ripgrep"),
	"rg":             WrapNewTool(githubprov.NewTool, "burntsushi/ripgrep"),
	"fd":             WrapNewTool(githubprov.NewTool, "sharkdp/fd"),
	"golangci-lint":  WrapNewTool(githubprov.NewTool, "golangci/golangci-lint"),
	"shellcheck":     WrapNewTool(githubprov.NewTool, "koalaman/shellcheck"),
	"actionlint":     WrapNewTool(githubprov.NewTool, "rhysd/actionlint"),
	"biome":          WrapNewTool(githubprov.NewTool, "biomejs/biome"),
	"shfmt":          WrapNewTool(githubprov.NewTool, "patrickvane/shfmt"),
	"sops":           WrapNewTool(githubprov.NewTool, "getsops/sops"),
	"docker-compose": WrapNewTool(githubprov.NewTool, "docker/compose"),
	"terraform":      WrapNewTool(githubprov.NewTool, "hashicorp/terraform"),
	"tflint":         WrapNewTool(githubprov.NewTool, "terraform-linters/tflint"),
	"opencode":       WrapNewTool(githubprov.NewTool, "anomalyco/opencode"),
	"rclone":         WrapNewTool(githubprov.NewTool, "rclone/rclone"),
	"rtk":            WrapNewTool(githubprov.NewTool, "rtk-ai/rtk"),
}
