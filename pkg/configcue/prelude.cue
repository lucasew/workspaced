package workspaced

// Prelude injected into every experimental workspaced.cue evaluation.
workspaced: {
	inputs: self: {
		from: *"self" | string
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
	backup: {
		rsyncnet_user: *"de3163@de3163.rsync.net" | string
		remote_path:   *"backup/lucasew" | string
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
