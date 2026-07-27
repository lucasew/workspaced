package workspaced

// Home-only prelude layer.
workspaced: {
	// Infrastructure: pin/install via the home/dotfiles lock, not per-codebase.
	lazy_tools: {
		mise: {
			ref:  *"registry:mise" | string
			bins: *["mise"] | [...string]
		}
	}
	workspaces: {
		www:  *1 | int
		meet: *2 | int
	}
	desktop: {
		wallpaper: {
			dir: *"\(workspaced.runtime.dotfiles_root)/assets/wallpapers" | string
		}
	}
	screenshot: {
		dir: *"\(workspaced.runtime.home)/Pictures/Screenshots" | string
	}
	quicksync: {
		repo_dir:    *"\(workspaced.runtime.home)/.personal" | string
		remote_path: *"/data2/home/de3163/git-personal" | string
	}
	browser: {
		default: *"zen" | string
		webapp:  *"brave" | string
	}
}
