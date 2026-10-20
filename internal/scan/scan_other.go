//go:build !unix && !windows

package scan

import "io/fs"

type fileKey struct{}

func hardLinkKey(fs.FileInfo) (fileKey, bool) { return fileKey{}, false }

func isReparseDir(fs.FileInfo) bool { return false }

func allocatedSize(_ string, fi fs.FileInfo) int64 { return fi.Size() }
