package apps

import (
	"workspaced/pkg/tool/backend"
	"workspaced/pkg/tool/backend/catalog"
	"workspaced/pkg/tool/backend/github"
)

var githubTools = map[string]string{
	"uv":             "astral-sh/uv",
	"fzf":            "junegunn/fzf",
	"ripgrep":        "burntsushi/ripgrep",
	"rg":             "burntsushi/ripgrep",
	"fd":             "sharkdp/fd",
	"sops":           "getsops/sops",
	"docker-compose": "docker/compose",
	"terraform":      "hashicorp/terraform",
	"tflint":         "terraform-linters/tflint",
	"opencode":       "anomalyco/opencode",
	"rclone":         "rclone/rclone",
	"rtk":            "rtk-ai/rtk",
	"resvg":          "linebender/resvg",
}

func init() {
	for name, repo := range githubTools {
		repo := repo
		catalog.RegisterTool(name, func() (backend.Tool, error) {
			return github.NewTool(repo)
		})
	}
}
