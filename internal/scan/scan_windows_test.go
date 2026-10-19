//go:build windows

package scan_test

import (
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/sys/windows"
)

func TestSparseFileAllocation(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "d", "f"), 1)
	f, err := os.Create(filepath.Join(root, "d", "sparse"))
	if err != nil {
		t.Fatal(err)
	}
	var ret uint32
	if err := windows.DeviceIoControl(windows.Handle(f.Fd()), windows.FSCTL_SET_SPARSE, nil, 0, nil, 0, &ret, nil); err != nil {
		f.Close()
		t.Skipf("FSCTL_SET_SPARSE: %v", err)
	}
	err = f.Truncate(sparseLen)
	// Close first: NTFS may report a stale size in the directory entry of an open file.
	f.Close()
	if err != nil {
		t.Fatal(err)
	}
	assertSparseFixture(t, root)
}
