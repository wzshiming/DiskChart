//go:build windows

package scan

import (
	"io/fs"
	"syscall"
)

type fileKey struct{}

func hardLinkKey(fs.FileInfo) (fileKey, bool) { return fileKey{}, false }

// isReparseDir reports a directory carrying a reparse point (junction, mount point).
func isReparseDir(fi fs.FileInfo) bool {
	d, ok := fi.Sys().(*syscall.Win32FileAttributeData)
	if !ok {
		return false
	}
	const want = syscall.FILE_ATTRIBUTE_DIRECTORY | syscall.FILE_ATTRIBUTE_REPARSE_POINT
	return d.FileAttributes&want == want
}
