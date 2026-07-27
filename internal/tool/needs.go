package tool

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ResolveNeedsCmd clones cmd, ensures each enabled needs entry via
// ResolveLazyToolAt, rewrites argv[0] when it matches a resolved binary, and
// returns PATH=… env extras for the child. needs may be nil.
func ResolveNeedsCmd(ctx context.Context, root string, cmd []string, needs map[string]bool) (argv []string, envExtra []string, err error) {
	argv = append([]string(nil), cmd...)
	if len(argv) == 0 {
		return nil, nil, nil
	}

	var pathDirs []string
	for name, on := range needs {
		if !on {
			continue
		}
		binName := filepath.Base(argv[0])
		binPath, rerr := ResolveLazyToolAt(ctx, root, name, binName)
		if rerr != nil {
			binPath, rerr = ResolveLazyToolAt(ctx, root, name, name)
		}
		if rerr != nil {
			return nil, nil, fmt.Errorf("ensure lazy tool %q: %w", name, rerr)
		}
		pathDirs = append(pathDirs, filepath.Dir(binPath))
		if filepath.Base(argv[0]) == filepath.Base(binPath) || argv[0] == name {
			argv[0] = binPath
		}
	}

	if len(pathDirs) > 0 {
		path := strings.Join(pathDirs, string(os.PathListSeparator))
		if existing := os.Getenv("PATH"); existing != "" {
			path = path + string(os.PathListSeparator) + existing
		}
		envExtra = append(envExtra, "PATH="+path)
	}
	return argv, envExtra, nil
}
