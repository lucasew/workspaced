package module

module: {
	meta: {
		name:        "example"
		description: "Example module demonstrating conditionals and features"
		version:     "1.0.0"
		requires:    []
		recommends:  []
	}

	config: {
		enable:        bool | *true
		greeting:      string | *"Hello from workspaced!"
		show_hostname: bool | *true
		show_ips:      bool | *true
	}

	drivers: {}
}
