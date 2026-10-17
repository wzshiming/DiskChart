//go:build unix

package scan

import (
	"io/fs"
	"syscall"
)

type fileKey struct{ dev, ino uint64 }

// hardLinkKey returns the identity of a regular file with more than one link.
func hardLinkKey(fi fs.FileInfo) (fileKey, bool) {
	if !fi.Mode().IsRegular() {
		return fileKey{}, false
	}
	st, ok := fi.Sys().(*syscall.Stat_t)
	if !ok || uint64(st.Nlink) <= 1 {
		return fileKey{}, false
	}
	return fileKey{uint64(st.Dev), uint64(st.Ino)}, true
}

func isReparseDir(fs.FileInfo) bool { return false }
