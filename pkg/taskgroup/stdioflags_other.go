//go:build !unix

package taskgroup

func snapshotStdioFlags() stdioFlags { return stdioFlags{} }

func (s stdioFlags) restore() {}
