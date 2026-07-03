package apps

import (
	"workspaced/pkg/tool/backend/catalog"
	"workspaced/pkg/tool/checks"
)

func init() {
	// name == on-disk binary
	for name, repo := range map[string]string{
		"uv":        "astral-sh/uv",
		"fzf":       "junegunn/fzf",
		"fd":        "sharkdp/fd",
		"sops":      "getsops/sops",
		"terraform": "hashicorp/terraform",
		"tflint":    "terraform-linters/tflint",
		"opencode":  "anomalyco/opencode",
		"rclone":    "rclone/rclone",
		"rtk":       "rtk-ai/rtk",
		"resvg":     "linebender/resvg",
	} {
		catalog.RegisterGitHub(name, repo)
	}

	// Aliases / binary name differs from catalog name.
	rgChecks := checks.Checks(checks.Binary("rg"))
	catalog.RegisterGitHub("ripgrep", "burntsushi/ripgrep", rgChecks...)
	catalog.RegisterGitHub("rg", "burntsushi/ripgrep", rgChecks...)
	catalog.RegisterGitHub("docker-compose", "docker/compose", checks.Binary("docker-compose"))
}
