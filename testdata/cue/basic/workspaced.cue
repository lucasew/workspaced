package workspaced

workspaced: {
	inputs: self: {
		from: "self"
	}

	modules: example: {
		input: "self"
		path:  "example"
		config: {
			enable: true
		}
	}
}
