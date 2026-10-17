//go:build unix

package scan_test

import (
	"context"
	"os"
	"path/filepath"
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
	if primary.Size != 300 || primary.Flags&scan.FlagHardLinkDup != 0 {
		t.Errorf("primary link %q = %+v, want Size 300 without FlagHardLinkDup", primary.Name, primary)
	}
	if dup.Size != 0 || dup.Flags&scan.FlagHardLinkDup == 0 {
		t.Errorf("duplicate link %q = %+v, want Size 0 with FlagHardLinkDup", dup.Name, dup)
	}
	if rn := mustNode(t, tree, tree.Root()); rn.Size != 300 {
		t.Errorf("root Size = %d, want 300", rn.Size)
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
	assertCounts(t, "root", mustNode(t, tree, tree.Root()), 1357, 5, 4)
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

	if b := mustNode(t, tree, find(t, tree, "b")); b.Size != 1000 {
		t.Errorf("b Size = %d, want 1000", b.Size)
	}
	if f2 := mustNode(t, tree, find(t, tree, "a", "f2")); f2.Size != 200 {
		t.Errorf("a/f2 Size = %d, want 200", f2.Size)
	}

	_, last := rec.snapshot()
	if last.Errors != int64(len(tree.Errors())) {
		t.Errorf("last Progress.Errors = %d, want len(Errors()) = %d", last.Errors, len(tree.Errors()))
	}
}
