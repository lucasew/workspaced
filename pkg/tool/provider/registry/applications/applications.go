package apps

import (
	"workspaced/pkg/tool/provider"
	"workspaced/pkg/tool/provider/github"
	"workspaced/pkg/tool/provider/registry"
)

func WrapNewTool(f func(ref string) (provider.Tool, error), ref string) func() (provider.Tool, error) {
	return func() (provider.Tool, error) {
		return f(ref)
	}
}

func init() {
	registry.RegisterRegistryTool("uv", WrapNewTool(github.NewTool, "astral-sh/uv"))
	registry.RegisterRegistryTool("ruff", WrapNewTool(github.NewTool, "astral-sh/ruff"))
	registry.RegisterRegistryTool("fzf", WrapNewTool(github.NewTool, "junegunn/fzf"))
	registry.RegisterRegistryTool("ripgrep", WrapNewTool(github.NewTool, "burntsushi/ripgrep"))
	registry.RegisterRegistryTool("rg", WrapNewTool(github.NewTool, "burntsushi/ripgrep"))
	registry.RegisterRegistryTool("fd", WrapNewTool(github.NewTool, "sharkdp/fd"))
	registry.RegisterRegistryTool("golangci-lint", WrapNewTool(github.NewTool, "golangci/golangci-lint"))
	registry.RegisterRegistryTool("shellcheck", WrapNewTool(github.NewTool, "koalaman/shellcheck"))
	registry.RegisterRegistryTool("actionlint", WrapNewTool(github.NewTool, "rhysd/actionlint"))
	registry.RegisterRegistryTool("biome", WrapNewTool(github.NewTool, "biomejs/biome"))
	registry.RegisterRegistryTool("shfmt", WrapNewTool(github.NewTool, "patrickvane/shfmt"))
	registry.RegisterRegistryTool("sops", WrapNewTool(github.NewTool, "getsops/sops"))
	registry.RegisterRegistryTool("docker-compose", WrapNewTool(github.NewTool, "docker/compose"))
	registry.RegisterRegistryTool("terraform", WrapNewTool(github.NewTool, "hashicorp/terraform"))
	registry.RegisterRegistryTool("tflint", WrapNewTool(github.NewTool, "terraform-linters/tflint"))
	registry.RegisterRegistryTool("opencode", WrapNewTool(github.NewTool, "anomalyco/opencode"))
	registry.RegisterRegistryTool("rclone", WrapNewTool(github.NewTool, "rclone/rclone"))
	registry.RegisterRegistryTool("rtk", WrapNewTool(github.NewTool, "rtk-ai/rtk"))
}
