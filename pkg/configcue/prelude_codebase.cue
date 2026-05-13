package workspaced

// Codebase-only prelude layer.
workspaced: {
	lazy_tools: {
		golangci_lint: {
			ref:  *"github:golangci/golangci-lint" | string
			bins: *["golangci-lint"] | [...string]
		}
		shellcheck: {
			ref:  *"github:koalaman/shellcheck" | string
			bins: *["shellcheck"] | [...string]
		}
		ruff: {
			ref:  *"github:astral-sh/ruff" | string
			bins: *["ruff"] | [...string]
		}
		actionlint: {
			ref:  *"github:rhysd/actionlint" | string
			bins: *["actionlint"] | [...string]
		}
		biome: {
			ref:  *"github:biomejs/biome" | string
			bins: *["biome"] | [...string]
		}
	}
}
