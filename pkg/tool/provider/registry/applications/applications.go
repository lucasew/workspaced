package apps

import (
	"workspaced/pkg/tool/provider"
	"workspaced/pkg/tool/provider/github"
	"workspaced/pkg/tool/provider/registry"
)

var githubTools = map[string]string{
	"uv":             "astral-sh/uv",
	"ruff":           "astral-sh/ruff",
	"fzf":            "junegunn/fzf",
	"ripgrep":        "burntsushi/ripgrep",
	"rg":             "burntsushi/ripgrep",
	"fd":             "sharkdp/fd",
	"golangci-lint":  "golangci/golangci-lint",
	"shellcheck":     "koalaman/shellcheck",
	"actionlint":     "rhysd/actionlint",
	"biome":          "biomejs/biome",
	"shfmt":          "patrickvane/shfmt",
	"sops":           "getsops/sops",
	"docker-compose": "docker/compose",
	"terraform":      "hashicorp/terraform",
	"tflint":         "terraform-linters/tflint",
	"opencode":       "anomalyco/opencode",
	"rclone":         "rclone/rclone",
	"rtk":            "rtk-ai/rtk",
}

func init() {
	for name, repo := range githubTools {
		repo := repo
		registry.RegisterRegistryTool(name, func() (provider.Tool, error) {
			return github.NewTool(repo)
		})
	}
}
