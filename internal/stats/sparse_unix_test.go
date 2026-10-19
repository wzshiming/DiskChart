//go:build unix

package stats_test

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/wzshiming/DiskChart/internal/scan"
	"github.com/wzshiming/DiskChart/internal/stats"
)

// sparseLen is the length of d/sparse, which stays a hole.
const sparseLen = 64 << 10

// sparseFixture builds d/big=16 units (sparseLen bytes, so it sorts before the hole), d/f=1 unit and the hole d/sparse.
func sparseFixture(t *testing.T) *scan.Tree {
	t.Helper()
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "d", "big"), 16)
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
	return scanOK(t, root)
}

func sparseEntry(t *testing.T, where string, es []stats.Entry) stats.Entry {
	t.Helper()
	for _, e := range es {
		if e.Name == "sparse" {
			return e
		}
	}
	t.Fatalf("%s = %+v, want an entry named sparse", where, es)
	return stats.Entry{}
}

func TestChildrenSparseApparent(t *testing.T) {
	tree := sparseFixture(t)
	e := sparseEntry(t, "Children(d)", mustChildren(t, tree, find(t, tree, "d"), 0, 10))
	if e.Apparent != sparseLen || e.Size >= e.Apparent {
		t.Errorf("Children(d) sparse Size/Apparent = %d/%d, want Apparent %d above Size", e.Size, e.Apparent, sparseLen)
	}
}

func TestTopFilesSparseApparent(t *testing.T) {
	tree := sparseFixture(t)
	if e := sparseEntry(t, "TopFiles(10)", stats.TopFiles(tree, 10)); e.Apparent != sparseLen {
		t.Errorf("TopFiles(10) sparse Apparent = %d, want %d", e.Apparent, sparseLen)
	}
}

func TestSubtreeSparseApparent(t *testing.T) {
	tree := sparseFixture(t)
	v := mustSubtree(t, tree, find(t, tree, "d"), 1, 0, 0)
	for _, c := range v.Children {
		if c.Name == "sparse" {
			if c.Apparent != sparseLen {
				t.Errorf("Subtree(d) sparse Apparent = %d, want %d", c.Apparent, sparseLen)
			}
			return
		}
	}
	t.Fatalf("Subtree(d).Children = %+v, want a view named sparse", v.Children)
}

func TestSubtreeSparseAggregate(t *testing.T) {
	tree := sparseFixture(t)
	v := mustSubtree(t, tree, find(t, tree, "d"), 1, 1, 0)
	// big is kept; f and the hole are merged.
	if agg := lastAggregate(t, "Subtree(d, 1, 1, 0)", v); agg.Count != 2 || agg.Apparent != sparseLen+unit {
		t.Errorf("d aggregate Count/Apparent = %d/%d, want 2/%d", agg.Count, agg.Apparent, sparseLen+unit)
	}
	var sum int64
	for _, c := range v.Children {
		sum += c.Apparent
	}
	if want := int64(sparseLen + 17*unit); v.Apparent != want || sum != want {
		t.Errorf("d Apparent = %d, Σ children.Apparent = %d, want both %d", v.Apparent, sum, want)
	}
	if n := checkChildrenMatch(t, tree, "d", v); n != 1 {
		t.Errorf("Subtree(d, 1, 1, 0) has %d aggregates, want 1", n)
	}
}
