package codebase

import pkg_config "workspaced/cmd/workspaced/codebase/config"

func init() {
	Registry.FromGetter(pkg_config.GetCommand)
}
