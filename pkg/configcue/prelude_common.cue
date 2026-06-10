package workspaced

// Common prelude injected into every workspaced.cue evaluation.
workspaced: {
	inputs: self: {
		from: *"self" | string
	}
	drivers: {
		"workspaced/pkg/driver/dialog.Chooser": {
			"terminal": *0 | int
		}
		"workspaced/pkg/driver/dialog.Confirmer": {
			"terminal": *0 | int
		}
		"workspaced/pkg/driver/dialog.Prompter": {
			"terminal": *0 | int
		}
		"workspaced/pkg/driver/svgraster.Driver": {
			"resvg": *100 | int
		}
	}
}
