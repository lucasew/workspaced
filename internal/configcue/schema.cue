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
	cpus?: int
	goos?: string
	goarch?: string
	memory?: int
	inputs?: [string]: {
		path?: string
		provider?: string
		target?: string
	}
}

#Concurrency: {
	io?:       int
	cpu?:      int
	internet?: int
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
	skip_permissions?: bool
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
	inputs: {
		self: #Input & {
			from: "self"
		}
		[string]: #Input
	}
	runtime?: #Runtime
	modules: {
		[string]: #ModuleRef
	}
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
	concurrency?: #Concurrency

	// LSP router: language servers behind `workspaced codebase lsp`.
	// Empty / omitted means the proxy still speaks LSP but routes nowhere.
	lsp?: #LSP
}

// #LSP is the codebase-local language server router config.
#LSP: {
	// extension (with or without leading dot) -> our language id
	extensions?: [string]: string
	// editor languageId -> our language id (used when extension map misses)
	language_ids?: [string]: string
	// our language -> ordered server attachments (keys like "00_gopls")
	languages?: [string]: #LSPLanguage
	// server id -> process definition
	servers?: [string]: #LSPServer
	// soft timeout per backend request (Go duration string), default applied in code
	request_timeout?: string
}

#LSPLanguage: {
	// ordered attachment key -> capabilities this server may handle for the language
	[string]: #LSPAttachment
}

#LSPAttachment: {
	// capability flags (hover, definition, diagnostics, …). Empty = all.
	capabilities?: [string]: bool
}

#LSPServer: {
	// argv for the language server (stdio)
	cmd: [...string] & [_, ...]
	// lazy_tools names to ensure before spawn (map for deep-merge)
	needs?: [string]: bool
}
