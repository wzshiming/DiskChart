//go:build unix

package app

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

// sparseLen is the length of the hole file sparse that sparseScan adds to the fixture root.
const sparseLen = 64 << 10

// sparseScan scans the fixture plus a sparseLen-byte hole named sparse, skipping if the file system fills holes.
func sparseScan(t *testing.T) (*App, ScanState) {
	t.Helper()
	root := fixture(t)
	p := filepath.Join(root, "sparse")
	f, err := os.Create(p)
	if err != nil {
		t.Fatal(err)
	}
	err = f.Truncate(sparseLen)
	f.Close()
	if err != nil {
		t.Fatal(err)
	}
	fi, err := os.Lstat(p)
	if err != nil {
		t.Fatal(err)
	}
	if blocks := fi.Sys().(*syscall.Stat_t).Blocks; blocks*512 >= sparseLen {
		t.Skipf("file system allocated %d blocks for a %d-byte hole", blocks, sparseLen)
	}

	rt := newFakeRuntime()
	a := New(rt, Deps{Mountpoints: noMounts})
	if err := a.StartScan(root, ScanOptions{}); err != nil {
		t.Fatalf("StartScan: %v", err)
	}
	st := waitFinished(t, a, rt)
	if st.Status != StatusDone {
		t.Fatalf("Status = %q (err %q), want %q", st.Status, st.Error, StatusDone)
	}
	return a, st
}

func TestChildrenSparseApparent(t *testing.T) {
	a, st := sparseScan(t)
	es, err := a.Children(st.RootID, 0, 10)
	if err != nil {
		t.Fatalf("Children(root): %v", err)
	}
	for _, e := range es {
		if e.Name == "sparse" {
			if e.Apparent != sparseLen || e.Size >= e.Apparent {
				t.Errorf("Children(root) sparse Size/Apparent = %d/%d, want Apparent %d above Size", e.Size, e.Apparent, sparseLen)
			}
			return
		}
	}
	t.Fatalf("Children(root) = %+v, want an entry named sparse", es)
}

func TestSubtreeSparseApparent(t *testing.T) {
	a, st := sparseScan(t)
	if v := child(t, a, st.RootID, "sparse"); v.Apparent != sparseLen || v.Size >= v.Apparent {
		t.Errorf("Subtree(root) sparse Size/Apparent = %d/%d, want Apparent %d above Size", v.Size, v.Apparent, sparseLen)
	}
}
