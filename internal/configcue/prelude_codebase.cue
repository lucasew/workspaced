package workspaced

// Codebase-only prelude layer.
workspaced: {
	lazy_tools: {
		golangci_lint: {
			ref:  *"github:golangci/golangci-lint" | string
			bins: *["golangci-lint"] | [...string]
		}
		shellcheck: {
			ref:  *"registry:shellcheck" | string
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
			ref:  *"registry:biome" | string
			bins: *["biome"] | [...string]
		}
		nodejs: {
			ref:  *"registry:nodejs" | string
			bins: *["node", "npm", "npx"] | [...string]
		}
	}

	lint: {
		tools: {
			"golangci-lint": {
				detect: {
					"00-go-mod": {path: "go.mod", enable: true}
				}
				needs: {golangci_lint: true}
				cmd: [
					"golangci-lint", "run",
					"--output.sarif.path=stdout",
					"--show-stats=false",
					"--issues-exit-code=0",
				]
				output: "sarif"
			}
			govulncheck: {
				detect: {
					"00-go-mod": {path: "go.mod", enable: true}
				}
				cmd: [
					"go", "run", "golang.org/x/vuln/cmd/govulncheck@v1.1.4",
					"--format", "sarif", "./...",
				]
				output: "sarif"
			}
			ruff: {
				detect: {
					"00-uv-lock": {path: "uv.lock", enable: true}
				}
				needs: {ruff: true}
				cmd: ["ruff", "check", "--output-format=sarif", "--exit-zero", "."]
				output: "sarif"
			}
			biome: {
				detect: {
					"00-package-json": {path: "package.json", enable: true}
				}
				needs: {biome: true}
				cmd: ["biome", "lint", "--reporter=sarif", "."]
				output: "sarif"
			}
			actionlint: {
				detect: {
					"00-workflows": {path: ".github/workflows", enable: true}
				}
				needs: {actionlint: true}
				cmd: ["actionlint", "-format", "{{json .}}"]
				output: "actionlint_json"
			}
			shellcheck: {
				detect: {
					"00-shell": {glob: "**/*.sh", enable: true}
				}
				needs: {shellcheck: true}
				cmd: ["shellcheck", "-f", "json"]
				output: "shellcheck_json"
				args_from_globs: true
			}
			eslint: {
				detect: {
					"00-eslint-bin": {path: "node_modules/.bin/eslint", enable: true}
				}
				cmd: ["node_modules/.bin/eslint", "-f", "json", "."]
				output: "eslint_json"
			}
		}
	}

	formatter: {
		tools: {
			gofmt: {
				detect: {
					"00-go-mod": {path: "go.mod", enable: true}
				}
				cmd: ["gofmt", "-w", "."]
			}
			ruff: {
				detect: {
					"00-uv-lock": {path: "uv.lock", enable: true}
				}
				needs: {ruff: true}
				cmd: ["ruff", "format", "."]
			}
			biome: {
				detect: {
					"00-package-json": {path: "package.json", enable: true}
				}
				needs: {biome: true}
				cmd: ["biome", "format", "--write", "."]
			}
			prettier: {
				detect: {
					"00-prettier-bin": {path: "node_modules/.bin/prettier", enable: true}
				}
				cmd: ["node_modules/.bin/prettier", "--write", "."]
			}
		}
	}
}
