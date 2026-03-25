package workspaced

#Host: {
	ips?:  [...string]
	mac?:  string
	port?: int
	user?: string
}

#LazyTool: {
	version?: string
	pkg?:     string
	global?:  bool
	alias?:   string
	bins?:    [...string]
}

#ModuleRef: {
	enable?: bool
	from?:   string
	[string]: _
}

workspaced: {
	workspaces?: [string]: int
	desktop?: {
		dark_mode?: bool
		wallpaper?: {
			dir?:     string
			default?: string
		}
	}
	screenshot?: {
		dir?: string
	}
	hosts?: [string]: #Host
	backup?: {
		rsyncnet_user?: string
		remote_path?:   string
	}
	quicksync?: {
		repo_dir?:    string
		remote_path?: string
	}
	browser?: {
		default?: string
		webapp?:  string
	}
	lazy_tools?: [string]: #LazyTool
	drivers?: [string]: [string]: int
	modules?: [string]: #ModuleRef
}
