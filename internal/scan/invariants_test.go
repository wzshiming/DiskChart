package scan_test

import (
	"path/filepath"
	"testing"

	"github.com/wzshiming/diskchart/internal/scan"
)

func checkInvariants(t *testing.T, tree *scan.Tree) {
	t.Helper()
	root := tree.Root()
	rn, ok := tree.Node(root)
	if !ok {
		t.Fatalf("Node(Root()=%d) not found", root)
	}
	if rn.Parent != scan.None {
		t.Errorf("root Parent = %d, want None", rn.Parent)
	}

	seen := map[scan.ID]bool{}
	var walk func(id scan.ID)
	walk = func(id scan.ID) {
		if seen[id] {
			t.Errorf("node %d reachable twice", id)
			return
		}
		seen[id] = true
		n, ok := tree.Node(id)
		if !ok {
			t.Errorf("Node(%d) listed as child but not found", id)
			return
		}
		if n.ID != id {
			t.Errorf("Node(%d).ID = %d", id, n.ID)
		}
		p := tree.Path(id)
		if !n.IsDir {
			if n.Children != nil {
				t.Errorf("non-directory %q has Children %v, want nil", p, n.Children)
			}
			return
		}

		var size, files, dirs int64
		var prev scan.Node
		for i, cid := range n.Children {
			c, ok := tree.Node(cid)
			if !ok {
				t.Errorf("child %d of %q not found", cid, p)
				continue
			}
			if c.Parent != id {
				t.Errorf("child %q Parent = %d, want %d", c.Name, c.Parent, id)
			}
			if got, want := tree.Path(cid), filepath.Join(p, c.Name); got != want {
				t.Errorf("Path(child) = %q, want %q", got, want)
			}
			size += c.Size
			if c.IsDir {
				files += c.FileCount
				dirs += 1 + c.DirCount
			} else {
				files++
			}
			if i > 0 && !(prev.Size > c.Size || prev.Size == c.Size && prev.Name < c.Name) {
				t.Errorf("Children of %q out of order: %q(%d) before %q(%d)", p, prev.Name, prev.Size, c.Name, c.Size)
			}
			prev = c
			walk(cid)
		}
		if n.Size != size {
			t.Errorf("%q Size = %d, want sum of children %d", p, n.Size, size)
		}
		if n.FileCount != files {
			t.Errorf("%q FileCount = %d, want %d", p, n.FileCount, files)
		}
		if n.DirCount != dirs {
			t.Errorf("%q DirCount = %d, want %d", p, n.DirCount, dirs)
		}
	}
	walk(root)

	if len(seen) != tree.Len() {
		t.Errorf("reachable nodes = %d, Len() = %d", len(seen), tree.Len())
	}
}
