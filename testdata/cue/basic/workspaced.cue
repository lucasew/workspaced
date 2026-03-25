package workspaced

workspaced: {
	inputs: exampleSource: {
		from:    "local:modules"
		version: "experimental"
	}

	modules: example: {
		input: "exampleSource"
		path:  "example"
		config: {
			enable: true
		}
	}
}
