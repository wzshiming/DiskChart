//go:build unix

package scan_test

import (
	"context"
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/wzshiming/DiskChart/internal/scan"
)

func TestHardLinkCountedOnce(t *testing.T) {
	root := t.TempDir()
	h1 := filepath.Join(root, "h1")
	writeFile(t, h1, 300)
	if err := os.Link(h1, filepath.Join(root, "h2")); err != nil {
		t.Fatal(err)
	}
	tree := scanOK(t, root, scan.Options{})
	checkInvariants(t, tree)

	n1 := mustNode(t, tree, find(t, tree, "h1"))
	n2 := mustNode(t, tree, find(t, tree, "h2"))
	primary, dup := n1, n2
	if n1.Flags&scan.FlagHardLinkDup != 0 {
		primary, dup = n2, n1
	}
	if primary.Size != 300*unit || primary.Flags&scan.FlagHardLinkDup != 0 {
		t.Errorf("primary link %q = %+v, want Size %d without FlagHardLinkDup", primary.Name, primary, 300*unit)
	}
	if dup.Size != 0 || dup.Flags&scan.FlagHardLinkDup == 0 {
		t.Errorf("duplicate link %q = %+v, want Size 0 with FlagHardLinkDup", dup.Name, dup)
	}
	if primary.Apparent != 300*unit || dup.Apparent != 0 {
		t.Errorf("Apparent of primary/duplicate link = %d/%d, want %d/0", primary.Apparent, dup.Apparent, 300*unit)
	}
	if rn := mustNode(t, tree, tree.Root()); rn.Size != 300*unit || rn.Apparent != 300*unit {
		t.Errorf("root Size/Apparent = %d/%d, want %d/%d", rn.Size, rn.Apparent, 300*unit, 300*unit)
	}
}

func TestSymlinkNotFollowed(t *testing.T) {
	base := t.TempDir()
	target := filepath.Join(base, "target")
	writeFile(t, filepath.Join(target, "x"), 10)
	writeFile(t, filepath.Join(target, "y"), 20)
	root := filepath.Join(base, "root")
	if err := os.Mkdir(root, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(root, "link")); err != nil {
		t.Fatal(err)
	}
	tree := scanOK(t, root, scan.Options{})
	checkInvariants(t, tree)

	l := mustNode(t, tree, find(t, tree, "link"))
	if l.IsDir || l.Flags&scan.FlagSymlink == 0 || l.Children != nil {
		t.Errorf("link = %+v, want non-dir with FlagSymlink and nil Children", l)
	}
	// The link's own Size depends on the file system, so only its length is pinned.
	if l.Apparent != int64(len(target)) {
		t.Errorf("link Apparent = %d, want len(target) = %d", l.Apparent, len(target))
	}
	if rn := mustNode(t, tree, tree.Root()); rn.FileCount != 1 || rn.DirCount != 0 {
		t.Errorf("root FileCount/DirCount = %d/%d, want 1/0 (symlink target not followed)", rn.FileCount, rn.DirCount)
	}
}

func TestRootSymlinkFollowed(t *testing.T) {
	root := fixture(t)
	link := filepath.Join(t.TempDir(), "rootlink")
	if err := os.Symlink(root, link); err != nil {
		t.Fatal(err)
	}
	tree := scanOK(t, link, scan.Options{})
	checkInvariants(t, tree)
	assertCounts(t, "root", mustNode(t, tree, tree.Root()), 1357*unit, 1357*unit, 5, 4)
}

func TestPermissionErrorRecorded(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores directory permissions")
	}
	root := fixture(t)
	subPath := filepath.Join(root, "a", "sub")
	if err := os.Chmod(subPath, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(subPath, 0o755) })

	var rec progressRecorder
	tree, err := scan.Scan(context.Background(), root, scan.Options{}, rec.on)
	if err != nil {
		t.Fatalf("Scan error: %v", err)
	}
	checkInvariants(t, tree)

	if sub := mustNode(t, tree, find(t, tree, "a", "sub")); sub.Flags&scan.FlagReadError == 0 {
		t.Errorf("a/sub Flags = %b, want FlagReadError", sub.Flags)
	}
	found := false
	for _, e := range tree.Errors() {
		if e.Path == subPath {
			found = true
		}
	}
	if !found {
		t.Errorf("Errors() = %+v, want an entry for %q", tree.Errors(), subPath)
	}

	if b := mustNode(t, tree, find(t, tree, "b")); b.Size != 1000*unit {
		t.Errorf("b Size = %d, want %d", b.Size, 1000*unit)
	}
	if f2 := mustNode(t, tree, find(t, tree, "a", "f2")); f2.Size != 200*unit {
		t.Errorf("a/f2 Size = %d, want %d", f2.Size, 200*unit)
	}

	_, last := rec.snapshot()
	if last.Errors != int64(len(tree.Errors())) {
		t.Errorf("last Progress.Errors = %d, want len(Errors()) = %d", last.Errors, len(tree.Errors()))
	}
}

func TestSparseFileAllocation(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "d", "f"), 1)
	p := filepath.Join(root, "d", "sparse")
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
	assertSparseFixture(t, root)
}
