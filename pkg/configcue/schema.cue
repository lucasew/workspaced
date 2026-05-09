package workspaced

#Input: {
	from:    string
	version?: string
}

#Host: {
	ips?:  [...string]
	mac?:  string
	port?: int
	user?: string
}

#LazyTool: {
	version?: string
	ref?:     string
	pkg?:     string
	global?:  bool
	alias?:   string
	bins?:    [...string]
}

#ModuleRef: {
	enable: bool | *true
	input?: string
	path?:  string | *"."
	from?:  string
	version?: string
	config?: _
}

#Runtime: {
	is_phone?: bool
	hostname?: string
	home?: string
	dotfiles_root?: string
	config_dir?: string
	user_data_dir?: string
	inputs?: [string]: {
		path?: string
		provider?: string
		target?: string
	}
}

#BackupActionGitRepoSync: close({
	name?: string
	kind: "git_repo_sync"
	src:  string
	dst:  string
})

#BackupActionRsync: close({
	name?: string
	kind: "rsync"
	src:  string
	dst:  string
	excludes?: [...string]
})

#BackupActionArchive: close({
	name?: string
	kind:      "archive"
	input_dir: string
	output:    string
	format:    "tar"
})


#BackupAction: #BackupActionGitRepoSync | #BackupActionRsync | #BackupActionArchive 

workspaced: {
	inputs?: {
		self?: #Input & {
			from: "self"
		}
		[string]: #Input
	}
	runtime?: #Runtime
	modules?: [string]: #ModuleRef
	workspaces?: [string]: int
	desktop?: {
		dark_mode?: bool
		raw?: {
			dconf?: [string]: [string]: _
		}
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
		git_repos?: [...{
			src: string
			dst: string
		}]
		actions?: [...#BackupAction]
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
}
