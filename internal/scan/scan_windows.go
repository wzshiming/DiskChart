//go:build windows

package scan

import (
	"io/fs"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
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

var procGetCompressedFileSizeW = windows.NewLazySystemDLL("kernel32.dll").NewProc("GetCompressedFileSizeW")

// allocatedSize asks GetCompressedFileSizeW about sparse or compressed files; other entries report their length.
func allocatedSize(path string, fi fs.FileInfo) int64 {
	d, ok := fi.Sys().(*syscall.Win32FileAttributeData)
	const want = windows.FILE_ATTRIBUTE_SPARSE_FILE | windows.FILE_ATTRIBUTE_COMPRESSED
	if !ok || d.FileAttributes&want == 0 || d.FileAttributes&windows.FILE_ATTRIBUTE_DIRECTORY != 0 {
		return fi.Size()
	}
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return fi.Size()
	}
	var high uint32
	r1, _, err := procGetCompressedFileSizeW.Call(uintptr(unsafe.Pointer(p)), uintptr(unsafe.Pointer(&high)))
	low := uint32(r1)
	// INVALID_FILE_SIZE is also a valid low word; only a nonzero last error marks failure.
	if errno, _ := err.(syscall.Errno); low == 0xFFFFFFFF && errno != 0 {
		return fi.Size()
	}
	return int64(uint64(high)<<32 | uint64(low))
}
