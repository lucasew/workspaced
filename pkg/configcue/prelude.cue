package workspaced

// Prelude injected into every experimental workspaced.cue evaluation.
workspaced: {
	inputs: self: {
		from: "self"
	}
}
