package rsync

import "strings"

// ValidatePaths returns ErrNeedsSrcAndDst when src or dst is blank.
func ValidatePaths(src, dst string) error {
	if strings.TrimSpace(src) == "" || strings.TrimSpace(dst) == "" {
		return ErrNeedsSrcAndDst
	}
	return nil
}

// BuildCLIArgs builds --exclude / --no-perms flags, then modeArgs (e.g. -avP), then src and dst.
func BuildCLIArgs(opts Options, modeArgs []string, src, dst string) []string {
	args := make([]string, 0, len(opts.Excludes)+len(modeArgs)+3)
	for _, x := range opts.Excludes {
		args = append(args, "--exclude="+x)
	}
	if opts.SkipPermissions {
		args = append(args, "--no-perms")
	}
	args = append(args, modeArgs...)
	return append(args, src, dst)
}
