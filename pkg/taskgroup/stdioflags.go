package taskgroup

// stdioFlags holds fcntl(F_GETFL) for stdin/stdout/stderr. Snapshot before
// bubbletea takes the tty; restore after it exits. On Darwin, tea sets
// O_NONBLOCK on the tty file description (stdout and stderr are usually
// dups of the same one), and does not clear it on teardown. Helix then
// unwraps write EAGAIN (errno 35).
type stdioFlags struct {
	fds []savedFD
}

type savedFD struct {
	fd    int
	flags int
}
