//go:build unix

package taskgroup

import "golang.org/x/sys/unix"

func snapshotStdioFlags() stdioFlags {
	var s stdioFlags
	for _, fd := range []int{unix.Stdin, unix.Stdout, unix.Stderr} {
		flags, err := unix.FcntlInt(uintptr(fd), unix.F_GETFL, 0)
		if err != nil {
			continue
		}
		s.fds = append(s.fds, savedFD{fd: fd, flags: flags})
	}
	return s
}

func (s stdioFlags) restore() {
	for _, saved := range s.fds {
		_, _ = unix.FcntlInt(uintptr(saved.fd), unix.F_SETFL, saved.flags)
	}
}

func fcntlGet(fd int) (int, error) {
	return unix.FcntlInt(uintptr(fd), unix.F_GETFL, 0)
}

func fcntlSet(fd, flags int) error {
	_, err := unix.FcntlInt(uintptr(fd), unix.F_SETFL, flags)
	return err
}

const oNonblock = unix.O_NONBLOCK
