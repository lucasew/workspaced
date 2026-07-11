package apps

import (
	"workspaced/internal/tool/backend/catalog"
	"workspaced/internal/tool/checks"
)

func init() {
	// name == on-disk binary
	for name, repo := range map[string]string{
		"uv":        "astral-sh/uv",
		"fzf":       "junegunn/fzf",
		"fd":        "sharkdp/fd",
		"sops":      "getsops/sops",
		"tflint":    "terraform-linters/tflint",
		"opencode":  "anomalyco/opencode",
		"rclone":    "rclone/rclone",
		"rtk":       "rtk-ai/rtk",
		"resvg":     "linebender/resvg",
		"codex":     "openai/codex",
		"contapila": "lucasew/contapila-go",
		"refactree": "lucasew/refactree",
	} {
		catalog.RegisterGitHub(name, repo)
	}

	// Aliases / binary name differs from catalog name.
	rgChecks := checks.Checks(checks.Binary("rg"))
	catalog.RegisterGitHub("ripgrep", "burntsushi/ripgrep", rgChecks...)
	catalog.RegisterGitHub("rg", "burntsushi/ripgrep", rgChecks...)
	catalog.RegisterGitHub("docker-compose", "docker/compose", checks.Binary("docker-compose"))
}
