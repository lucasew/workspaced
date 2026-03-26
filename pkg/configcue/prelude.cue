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
	drivers: {
		"workspaced/pkg/driver/clipboard.Driver": {
			"clipboard_termux": *60 | int
		}
		"workspaced/pkg/driver/dialog.Chooser": {
			"terminal": *0 | int
		}
		"workspaced/pkg/driver/dialog.Confirmer": {
			"terminal": *0 | int
		}
		"workspaced/pkg/driver/dialog.Prompter": {
			"terminal": *0 | int
		}
		"workspaced/pkg/driver/env.Driver": {
			"env_termux": *60 | int
		}
		"workspaced/pkg/driver/exec.Driver": {
			"exec_termux": *60 | int
		}
		"workspaced/pkg/driver/httpclient.Driver": {
			"httpclient_termux": *60 | int
		}
		"workspaced/pkg/driver/notification.Driver": {
			"notification_termux": *60 | int
		}
		"workspaced/pkg/driver/opener.Driver": {
			"opener_termux": *60 | int
		}
		"workspaced/pkg/driver/power.Driver": {
			"power_termux": *60 | int
		}
		"workspaced/pkg/driver/svgraster.Driver": {
			"resvg": *100 | int
		}
		"workspaced/pkg/driver/terminal.Driver": {
			"terminal_termux": *60 | int
		}
	}
}
