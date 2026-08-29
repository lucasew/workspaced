package apps

import (
	"github.com/lucasew/workspaced/internal/tool/backend/catalog"
	"github.com/lucasew/workspaced/internal/tool/checks"
)

func init() {
	catalog.RegisterGitHub("uv", "astral-sh/uv", checks.Binary("uv"))
	catalog.RegisterGitHub("fzf", "junegunn/fzf", checks.Binary("fzf"))
	catalog.RegisterGitHub("fd", "sharkdp/fd", checks.Binary("fd"))
	catalog.RegisterGitHub("sops", "getsops/sops", checks.Binary("sops"))
	catalog.RegisterGitHub("tflint", "terraform-linters/tflint", checks.Binary("tflint"))
	catalog.RegisterGitHub("opencode", "anomalyco/opencode", checks.Binary("opencode"))
	catalog.RegisterGitHub("rclone", "rclone/rclone", checks.Binary("rclone"))
	catalog.RegisterGitHub("rtk", "rtk-ai/rtk", checks.Binary("rtk"))
	catalog.RegisterGitHub("resvg", "linebender/resvg", checks.Binary("resvg"))
	catalog.RegisterGitHub("codex", "openai/codex", checks.Binary("codex"))
	catalog.RegisterGitHub("contapila", "lucasew/contapila", checks.Binary("contapila"))
	catalog.RegisterGitHub("docker-compose", "docker/compose", checks.Binary("docker-compose"))
	catalog.RegisterGitHub("refactree", "lucasew/refactree", checks.Binary("rft"))
	catalog.RegisterGitHub("ripgrep", "burntsushi/ripgrep", checks.Binary("rg"))
}
