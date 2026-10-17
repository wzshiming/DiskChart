package scan_test

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/wzshiming/DiskChart/internal/scan"
)

func writeFile(t *testing.T, p string, size int) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, make([]byte, size), 0o644); err != nil {
		t.Fatal(err)
	}
}

// fixture: a/f1=100 a/f2=200 a/sub/f3=50 b/f4=1000 empty/ r.txt=7
func fixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "a", "f1"), 100)
	writeFile(t, filepath.Join(root, "a", "f2"), 200)
	writeFile(t, filepath.Join(root, "a", "sub", "f3"), 50)
	writeFile(t, filepath.Join(root, "b", "f4"), 1000)
	if err := os.Mkdir(filepath.Join(root, "empty"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(root, "r.txt"), 7)
	return root
}

func scanOK(t *testing.T, root string, opts scan.Options) *scan.Tree {
	t.Helper()
	tree, err := scan.Scan(context.Background(), root, opts, nil)
	if err != nil {
		t.Fatalf("Scan(%q) error: %v", root, err)
	}
	if tree == nil {
		t.Fatalf("Scan(%q) returned nil tree", root)
	}
	return tree
}

func find(t *testing.T, tree *scan.Tree, names ...string) scan.ID {
	t.Helper()
	id := tree.Root()
	for _, name := range names {
		n := mustNode(t, tree, id)
		found := scan.None
		for _, c := range n.Children {
			if cn, ok := tree.Node(c); ok && cn.Name == name {
				found = c
				break
			}
		}
		if found == scan.None {
			t.Fatalf("child %q not found under %q", name, tree.Path(id))
		}
		id = found
	}
	return id
}

func mustNode(t *testing.T, tree *scan.Tree, id scan.ID) scan.Node {
	t.Helper()
	n, ok := tree.Node(id)
	if !ok {
		t.Fatalf("Node(%d) not found", id)
	}
	return n
}

func childNames(t *testing.T, tree *scan.Tree, id scan.ID) []string {
	t.Helper()
	var names []string
	for _, c := range mustNode(t, tree, id).Children {
		names = append(names, mustNode(t, tree, c).Name)
	}
	return names
}

func assertCounts(t *testing.T, label string, n scan.Node, size, files, dirs int64) {
	t.Helper()
	if n.Size != size || n.FileCount != files || n.DirCount != dirs {
		t.Errorf("%s: Size/FileCount/DirCount = %d/%d/%d, want %d/%d/%d",
			label, n.Size, n.FileCount, n.DirCount, size, files, dirs)
	}
}

func TestScanFixtureAggregates(t *testing.T) {
	root := fixture(t)
	tree := scanOK(t, root, scan.Options{})
	checkInvariants(t, tree)

	rn := mustNode(t, tree, tree.Root())
	assertCounts(t, "root", rn, 1357, 5, 4)
	if !rn.IsDir {
		t.Error("root IsDir = false")
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Clean(abs); rn.Name != want {
		t.Errorf("root Name = %q, want %q", rn.Name, want)
	}

	assertCounts(t, "a", mustNode(t, tree, find(t, tree, "a")), 350, 3, 1)

	if got, want := childNames(t, tree, tree.Root()), []string{"b", "a", "r.txt", "empty"}; !slices.Equal(got, want) {
		t.Errorf("root Children names = %v, want %v", got, want)
	}

	f3 := find(t, tree, "a", "sub", "f3")
	if got, want := tree.Path(f3), filepath.Join(root, "a", "sub", "f3"); got != want {
		t.Errorf("Path(f3) = %q, want %q", got, want)
	}
	if n := mustNode(t, tree, f3); n.IsDir || n.Size != 50 || n.Children != nil {
		t.Errorf("f3 = %+v, want file of Size 50 with nil Children", n)
	}
}

func TestNodeNoneNotFound(t *testing.T) {
	tree := scanOK(t, fixture(t), scan.Options{})
	if _, ok := tree.Node(scan.None); ok {
		t.Error("Node(None) ok = true, want false")
	}
	if p := tree.Path(scan.None); p != "" {
		t.Errorf("Path(None) = %q, want empty", p)
	}
}

func TestScanTwiceStable(t *testing.T) {
	root := fixture(t)
	t1 := scanOK(t, root, scan.Options{})
	t2 := scanOK(t, root, scan.Options{})
	if t1.Len() != t2.Len() {
		t.Errorf("Len differs: %d vs %d", t1.Len(), t2.Len())
	}
	if s1, s2 := mustNode(t, t1, t1.Root()).Size, mustNode(t, t2, t2.Root()).Size; s1 != s2 {
		t.Errorf("root Size differs: %d vs %d", s1, s2)
	}
	if n1, n2 := childNames(t, t1, t1.Root()), childNames(t, t2, t2.Root()); !slices.Equal(n1, n2) {
		t.Errorf("root Children names differ: %v vs %v", n1, n2)
	}
}

func TestScanRootIsFileErrNotDir(t *testing.T) {
	p := filepath.Join(t.TempDir(), "file")
	writeFile(t, p, 10)
	_, err := scan.Scan(context.Background(), p, scan.Options{}, nil)
	if !errors.Is(err, scan.ErrNotDir) {
		t.Errorf("Scan(file) error = %v, want ErrNotDir", err)
	}
}

func TestScanRootMissing(t *testing.T) {
	tree, err := scan.Scan(context.Background(), filepath.Join(t.TempDir(), "missing"), scan.Options{}, nil)
	if err == nil {
		t.Error("Scan(missing) error = nil, want non-nil")
	}
	if tree != nil {
		t.Error("Scan(missing) tree != nil, want nil")
	}
}

func TestMountPointExcludedWhenSameDevice(t *testing.T) {
	root := fixture(t)
	tree := scanOK(t, root, scan.Options{
		SameDevice:         true,
		ExcludeMountpoints: []string{filepath.Join(root, "b")},
	})
	checkInvariants(t, tree)

	b := mustNode(t, tree, find(t, tree, "b"))
	if !b.IsDir || b.Flags&scan.FlagMountPoint == 0 || b.Size != 0 || len(b.Children) != 0 {
		t.Errorf("b = %+v, want IsDir, FlagMountPoint, Size 0, no Children", b)
	}
	assertCounts(t, "root", mustNode(t, tree, tree.Root()), 357, 4, 4)
}

func TestMountPointEnteredWhenNotSameDevice(t *testing.T) {
	root := fixture(t)
	tree := scanOK(t, root, scan.Options{
		SameDevice:         false,
		ExcludeMountpoints: []string{filepath.Join(root, "b")},
	})
	checkInvariants(t, tree)

	b := mustNode(t, tree, find(t, tree, "b"))
	if b.Flags&scan.FlagMountPoint != 0 || b.Size != 1000 || len(b.Children) != 1 {
		t.Errorf("b = %+v, want entered normally with Size 1000", b)
	}
	assertCounts(t, "root", mustNode(t, tree, tree.Root()), 1357, 5, 4)
}

// progressRecorder detects concurrent callbacks via TryLock and keeps the last report.
type progressRecorder struct {
	mu         sync.Mutex
	calls      int
	last       scan.Progress
	concurrent atomic.Int64
}

func (r *progressRecorder) on(p scan.Progress) {
	if !r.mu.TryLock() {
		r.concurrent.Add(1)
		return
	}
	defer r.mu.Unlock()
	r.calls++
	r.last = p
}

func (r *progressRecorder) snapshot() (int, scan.Progress) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.calls, r.last
}

func TestProgressFinalMatchesTree(t *testing.T) {
	var rec progressRecorder
	tree, err := scan.Scan(context.Background(), fixture(t), scan.Options{}, rec.on)
	if err != nil {
		t.Fatal(err)
	}
	if n := rec.concurrent.Load(); n != 0 {
		t.Errorf("onProgress called concurrently %d times", n)
	}
	calls, last := rec.snapshot()
	if calls < 1 {
		t.Fatal("onProgress never called")
	}
	rn := mustNode(t, tree, tree.Root())
	if last.Dirs != rn.DirCount+1 || last.Files != rn.FileCount || last.Bytes != rn.Size || last.Errors != int64(len(tree.Errors())) {
		t.Errorf("last Progress = %+v, want Dirs %d Files %d Bytes %d Errors %d",
			last, rn.DirCount+1, rn.FileCount, rn.Size, len(tree.Errors()))
	}
}

// bigFixture creates 60 directories of 60 empty files each.
func bigFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for d := range 60 {
		dir := filepath.Join(root, fmt.Sprintf("d%02d", d))
		if err := os.Mkdir(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		for f := range 60 {
			if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("f%02d", f)), nil, 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
	return root
}

func TestProgressSpacing(t *testing.T) {
	root := bigFixture(t)
	var times []time.Time
	_, err := scan.Scan(context.Background(), root, scan.Options{}, func(scan.Progress) {
		times = append(times, time.Now())
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(times) < 2 {
		t.Skipf("only %d onProgress call(s): scan finished before the first interval", len(times))
	}
	for i := 1; i < len(times); i++ {
		if d := times[i].Sub(times[i-1]); d < 99*time.Millisecond {
			t.Errorf("onProgress calls %d and %d entered %v apart, want >= 100ms", i-1, i, d)
		}
	}
}

func TestScanCancelMidway(t *testing.T) {
	root := bigFixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	tree, err := scan.Scan(ctx, root, scan.Options{}, func(scan.Progress) { cancel() })
	if err != nil && !errors.Is(err, context.Canceled) {
		t.Errorf("Scan canceled in onProgress: err = %v, want nil or context.Canceled", err)
	}
	if tree == nil {
		t.Fatal("Scan canceled in onProgress: tree = nil")
	}
	checkInvariants(t, tree)
}

func TestProgressNilCallback(t *testing.T) {
	tree := scanOK(t, fixture(t), scan.Options{})
	checkInvariants(t, tree)
}

func TestScanCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	tree, err := scan.Scan(ctx, fixture(t), scan.Options{}, nil)
	if !errors.Is(err, context.Canceled) {
		t.Errorf("Scan(canceled) error = %v, want context.Canceled", err)
	}
	if tree == nil {
		t.Fatal("Scan(canceled) tree = nil, want partial tree")
	}
	if n := mustNode(t, tree, tree.Root()); !n.IsDir {
		t.Error("canceled tree root IsDir = false")
	}
	checkInvariants(t, tree)
}

func TestRemoveUpdatesAncestors(t *testing.T) {
	tree := scanOK(t, fixture(t), scan.Options{})
	a := find(t, tree, "a")
	sub := find(t, tree, "a", "sub")
	f3 := find(t, tree, "a", "sub", "f3")
	before := tree.Len()

	parent, err := tree.Remove(sub)
	if err != nil {
		t.Fatalf("Remove(a/sub) error: %v", err)
	}
	if parent != a {
		t.Errorf("Remove(a/sub) parent = %d, want a=%d", parent, a)
	}
	assertCounts(t, "a", mustNode(t, tree, a), 300, 2, 0)
	assertCounts(t, "root", mustNode(t, tree, tree.Root()), 1307, 4, 3)
	if _, ok := tree.Node(sub); ok {
		t.Error("Node(sub) ok after Remove")
	}
	if _, ok := tree.Node(f3); ok {
		t.Error("Node(f3) ok after removing its parent")
	}
	if got := tree.Len(); got != before-2 {
		t.Errorf("Len = %d, want %d", got, before-2)
	}
	checkInvariants(t, tree)

	b := find(t, tree, "b")
	if parent, err := tree.Remove(b); err != nil || parent != tree.Root() {
		t.Errorf("Remove(b) = %d, %v; want Root(), nil", parent, err)
	}
	checkInvariants(t, tree)
	if got, want := childNames(t, tree, tree.Root()), []string{"a", "r.txt", "empty"}; !slices.Equal(got, want) {
		t.Errorf("root Children after Remove(b) = %v, want %v", got, want)
	}

	r := find(t, tree, "r.txt")
	if parent, err := tree.Remove(r); err != nil || parent != tree.Root() {
		t.Errorf("Remove(r.txt) = %d, %v; want Root(), nil", parent, err)
	}
	checkInvariants(t, tree)
	if got, want := childNames(t, tree, tree.Root()), []string{"a", "empty"}; !slices.Equal(got, want) {
		t.Errorf("root Children after Remove(r.txt) = %v, want %v", got, want)
	}
	assertCounts(t, "root", mustNode(t, tree, tree.Root()), 300, 2, 2)
}

func TestRemoveFileReturnsParent(t *testing.T) {
	tree := scanOK(t, fixture(t), scan.Options{})
	a := find(t, tree, "a")
	f1 := find(t, tree, "a", "f1")
	parent, err := tree.Remove(f1)
	if err != nil || parent != a {
		t.Errorf("Remove(a/f1) = %d, %v; want a=%d, nil", parent, err, a)
	}
	assertCounts(t, "a", mustNode(t, tree, a), 250, 2, 1)
	checkInvariants(t, tree)
}

func TestRemoveRootErrRoot(t *testing.T) {
	tree := scanOK(t, fixture(t), scan.Options{})
	if _, err := tree.Remove(tree.Root()); !errors.Is(err, scan.ErrRoot) {
		t.Errorf("Remove(Root()) error = %v, want ErrRoot", err)
	}
	checkInvariants(t, tree)
}

func TestRemoveInvalidID(t *testing.T) {
	tree := scanOK(t, fixture(t), scan.Options{})
	for _, id := range []scan.ID{scan.ID(1 << 30), scan.None} {
		if _, err := tree.Remove(id); !errors.Is(err, scan.ErrInvalidID) {
			t.Errorf("Remove(%d) error = %v, want ErrInvalidID", id, err)
		}
	}
}

func TestRandomTreeInvariants(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	root := t.TempDir()
	type dir struct {
		p     string
		depth int
	}
	dirs := []dir{{root, 0}}
	for i := 0; i < 40; {
		parent := dirs[rng.Intn(len(dirs))]
		if parent.depth >= 3 {
			continue
		}
		d := dir{filepath.Join(parent.p, fmt.Sprintf("d%d", i)), parent.depth + 1}
		if err := os.Mkdir(d.p, 0o755); err != nil {
			t.Fatal(err)
		}
		dirs = append(dirs, d)
		i++
	}
	var total int64
	for i := range 200 {
		size := rng.Intn(5001)
		total += int64(size)
		writeFile(t, filepath.Join(dirs[rng.Intn(len(dirs))].p, fmt.Sprintf("f%d", i)), size)
	}

	var rec progressRecorder
	tree, err := scan.Scan(context.Background(), root, scan.Options{}, rec.on)
	if err != nil {
		t.Fatal(err)
	}
	checkInvariants(t, tree)
	assertCounts(t, "root", mustNode(t, tree, tree.Root()), total, 200, 40)
	if n := rec.concurrent.Load(); n != 0 {
		t.Errorf("onProgress called concurrently %d times", n)
	}
}
