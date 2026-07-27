package terminal

// BuildOpenArgs builds title and optional command argv fragments for terminal Open.
// titleFlag is e.g. "-T" or "--title". When commandAsE is true, the command is
// passed as "-e", command (alacritty); otherwise as bare command (foot/kitty).
func BuildOpenArgs(opts Options, titleFlag string, commandAsE bool) []string {
	args := make([]string, 0, 4+len(opts.Args))
	if opts.Title != "" {
		args = append(args, titleFlag, opts.Title)
	}
	if opts.Command != "" {
		if commandAsE {
			args = append(args, "-e", opts.Command)
		} else {
			args = append(args, opts.Command)
		}
		args = append(args, opts.Args...)
	}
	return args
}
