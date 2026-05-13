package workspaced

// Common prelude injected into every workspaced.cue evaluation.
workspaced: {
	inputs: self: {
		from: *"self" | string
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
