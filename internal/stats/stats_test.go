package stats_test

import (
	"cmp"
	"context"
	"fmt"
	"math/rand/v2"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/wzshiming/DiskChart/internal/scan"
	"github.com/wzshiming/DiskChart/internal/stats"
)

// unit is one 4 KiB block: whole units of random data occupy exactly their length on APFS, ext4 and NTFS.
const unit = 4096

// writeFile writes size units of fixed-seed random bytes, which no file system can compress.
func writeFile(t *testing.T, p string, size int) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	b := make([]byte, size*unit)
	rand.NewChaCha8([32]byte{}).Read(b)
	if err := os.WriteFile(p, b, 0o644); err != nil {
		t.Fatal(err)
	}
}

// fixture builds (sizes in units, total 7192):
//
//	a/f1=100 a/f2=200 a/sub/f3=50          a=350 sub=50
//	b/f4=1000                              b=1000
//	empty/                                 empty=0
//	r.txt=7
//	x/y/z/big.bin=5000 x/small.txt=10      x=5010 y=5000 z=5000
//	docs/report.PDF=300 docs/.hidden=5 docs/noext=20   docs=325
//	pics/a.JPG=400 pics/b.jpg=100          pics=500
func fixture(t *testing.T) (*scan.Tree, string) {
	t.Helper()
	root := t.TempDir()
	for p, size := range map[string]int{
		"a/f1":            100,
		"a/f2":            200,
		"a/sub/f3":        50,
		"b/f4":            1000,
		"r.txt":           7,
		"x/y/z/big.bin":   5000,
		"x/small.txt":     10,
		"docs/report.PDF": 300,
		"docs/.hidden":    5,
		"docs/noext":      20,
		"pics/a.JPG":      400,
		"pics/b.jpg":      100,
	} {
		writeFile(t, filepath.Join(root, filepath.FromSlash(p)), size)
	}
	if err := os.Mkdir(filepath.Join(root, "empty"), 0o755); err != nil {
		t.Fatal(err)
	}
	return scanOK(t, root), root
}

func scanOK(t *testing.T, root string) *scan.Tree {
	t.Helper()
	tree, err := scan.Scan(context.Background(), root, scan.Options{}, nil)
	if err != nil {
		t.Fatalf("Scan(%q) error: %v", root, err)
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

func entry(t *testing.T, tree *scan.Tree, names ...string) stats.Entry {
	t.Helper()
	id := find(t, tree, names...)
	n := mustNode(t, tree, id)
	e := stats.Entry{ID: id, Name: n.Name, RelPath: filepath.Join(names...), Size: n.Size, Apparent: n.Apparent, IsDir: n.IsDir, ModTime: n.ModTime}
	if n.IsDir {
		e.FileCount = n.FileCount
	}
	return e
}

func checkEntries(t *testing.T, fn string, got, want []stats.Entry) {
	t.Helper()
	if !slices.Equal(got, want) {
		t.Errorf("%s:\n got  %+v\n want %+v", fn, got, want)
	}
}

func TestTopFiles(t *testing.T) {
	tree, _ := fixture(t)

	want := []stats.Entry{
		entry(t, tree, "x", "y", "z", "big.bin"), // 5000
		entry(t, tree, "b", "f4"),                // 1000
		entry(t, tree, "pics", "a.JPG"),          // 400
	}
	checkEntries(t, "TopFiles(3)", stats.TopFiles(tree, 3), want)

	// Ties on Size break by RelPath ascending: a/f1 before pics/b.jpg (both 100).
	all := []stats.Entry{
		entry(t, tree, "x", "y", "z", "big.bin"), // 5000
		entry(t, tree, "b", "f4"),                // 1000
		entry(t, tree, "pics", "a.JPG"),          // 400
		entry(t, tree, "docs", "report.PDF"),     // 300
		entry(t, tree, "a", "f2"),                // 200
		entry(t, tree, "a", "f1"),                // 100
		entry(t, tree, "pics", "b.jpg"),          // 100
		entry(t, tree, "a", "sub", "f3"),         // 50
		entry(t, tree, "docs", "noext"),          // 20
		entry(t, tree, "x", "small.txt"),         // 10
		entry(t, tree, "r.txt"),                  // 7
		entry(t, tree, "docs", ".hidden"),        // 5
	}
	checkEntries(t, "TopFiles(100)", stats.TopFiles(tree, 100), all)
}

func TestTopFilesZero(t *testing.T) {
	tree, _ := fixture(t)
	for _, n := range []int{0, -1} {
		if got := stats.TopFiles(tree, n); len(got) != 0 {
			t.Errorf("TopFiles(%d) = %+v, want empty", n, got)
		}
	}
}

func TestTopFilesIncludesLinks(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "f"), 10)
	if err := os.Symlink("f", filepath.Join(root, "l")); err != nil {
		t.Skipf("symlink unsupported: %v", err)
	}
	if err := os.Link(filepath.Join(root, "f"), filepath.Join(root, "h")); err != nil {
		t.Skipf("hard link unsupported: %v", err)
	}
	tree := scanOK(t, root)

	got := stats.TopFiles(tree, 10)
	var names []string
	for _, e := range got {
		names = append(names, e.Name)
	}
	slices.Sort(names)
	if !slices.Equal(names, []string{"f", "h", "l"}) {
		t.Errorf("TopFiles names = %v, want [f h l] (symlink and hard-link dup included)", names)
	}
}

func TestTopDirsDominance(t *testing.T) {
	if stats.DominanceThreshold != 0.9 {
		t.Errorf("DominanceThreshold = %v, want 0.9", stats.DominanceThreshold)
	}
	tree, _ := fixture(t)

	// Excluded: root; empty (Size 0); x (y=5000 >= 0.9*5010); y (z=5000 >= 0.9*5000).
	// a keeps: its only subdir sub=50 < 0.9*350.
	want := []stats.Entry{
		entry(t, tree, "x", "y", "z"), // 5000
		entry(t, tree, "b"),           // 1000
		entry(t, tree, "pics"),        // 500
		entry(t, tree, "a"),           // 350
		entry(t, tree, "docs"),        // 325
		entry(t, tree, "a", "sub"),    // 50
	}
	checkEntries(t, "TopDirs(10)", stats.TopDirs(tree, 10), want)
	checkEntries(t, "TopDirs(2)", stats.TopDirs(tree, 2), want[:2])
}

func TestTopDirsZero(t *testing.T) {
	tree, _ := fixture(t)
	for _, n := range []int{0, -1} {
		if got := stats.TopDirs(tree, n); len(got) != 0 {
			t.Errorf("TopDirs(%d) = %+v, want empty", n, got)
		}
	}
}

func ext(e string, size, count int64) stats.ExtStat {
	return stats.ExtStat{Ext: e, Category: stats.CategoryOf(e), Size: size, Count: count}
}

// Per extension: bin 5000/1; "" = f1+f2+f3+f4+.hidden+noext = 1375/6;
// jpg = 400+100 = 500/2; pdf 300/1; txt = 7+10 = 17/2.
var allExts = []stats.ExtStat{
	ext("bin", 5000*unit, 1),
	ext("", 1375*unit, 6),
	ext("jpg", 500*unit, 2),
	ext("pdf", 300*unit, 1),
	ext("txt", 17*unit, 2),
}

func TestByExtension(t *testing.T) {
	tree, _ := fixture(t)
	got := stats.ByExtension(tree, 2)

	if !slices.Equal(got.Items, allExts[:2]) {
		t.Errorf("ByExtension(2).Items:\n got  %+v\n want %+v", got.Items, allExts[:2])
	}
	// Other = jpg+pdf+txt = 500+300+17 = 817, Count 2+1+2 = 5.
	wantOther := stats.ExtStat{Ext: "", Category: "other", Size: 817 * unit, Count: 5}
	if got.Other != wantOther {
		t.Errorf("ByExtension(2).Other = %+v, want %+v", got.Other, wantOther)
	}
	if got.Total != 7192*unit {
		t.Errorf("ByExtension(2).Total = %d, want %d", got.Total, 7192*unit)
	}
}

func TestByExtensionAll(t *testing.T) {
	tree, _ := fixture(t)
	for _, k := range []int{0, -1, len(allExts), len(allExts) + 5} {
		got := stats.ByExtension(tree, k)
		if !slices.Equal(got.Items, allExts) {
			t.Errorf("ByExtension(%d).Items:\n got  %+v\n want %+v", k, got.Items, allExts)
		}
		if want := (stats.ExtStat{Ext: "", Category: "other"}); got.Other != want {
			t.Errorf("ByExtension(%d).Other = %+v, want %+v", k, got.Other, want)
		}
		if got.Total != 7192*unit {
			t.Errorf("ByExtension(%d).Total = %d, want %d", k, got.Total, 7192*unit)
		}
	}
}

func TestCategoryOf(t *testing.T) {
	cases := map[string][]string{
		"images":    {"jpg", "png", "heic"},
		"video":     {"mp4", "mov", "mkv", "ts"},
		"audio":     {"mp3", "flac"},
		"archives":  {"zip", "tar", "gz", "dmg"},
		"code":      {"go", "py", "json", "md"},
		"documents": {"pdf", "docx", "txt", "csv"},
		"apps":      {"app", "exe", "dll", "dylib"},
		"other":     {"", "xyz"},
	}
	for want, exts := range cases {
		for _, e := range exts {
			if got := stats.CategoryOf(e); got != want {
				t.Errorf("CategoryOf(%q) = %q, want %q", e, got, want)
			}
		}
	}
}

func view(t *testing.T, tree *scan.Tree, id scan.ID) stats.View {
	t.Helper()
	n := mustNode(t, tree, id)
	return stats.View{ID: n.ID, Name: n.Name, Size: n.Size, Apparent: n.Apparent, IsDir: n.IsDir, FileCount: n.FileCount}
}

// checkView compares the node fields of got against want, ignoring Children.
func checkView(t *testing.T, where string, got, want stats.View) {
	t.Helper()
	if got.ID != want.ID || got.Name != want.Name || got.Size != want.Size || got.IsDir != want.IsDir || got.Aggregate != want.Aggregate {
		t.Errorf("%s = {ID:%d Name:%q Size:%d IsDir:%v Aggregate:%v}, want {ID:%d Name:%q Size:%d IsDir:%v Aggregate:%v}",
			where, got.ID, got.Name, got.Size, got.IsDir, got.Aggregate, want.ID, want.Name, want.Size, want.IsDir, want.Aggregate)
	}
	if got.Apparent != want.Apparent {
		t.Errorf("%s.Apparent = %d, want %d", where, got.Apparent, want.Apparent)
	}
	if want.IsDir && got.FileCount != want.FileCount {
		t.Errorf("%s.FileCount = %d, want %d", where, got.FileCount, want.FileCount)
	}
}

func checkAggregate(t *testing.T, where string, got stats.View, size, apparent, count, files int64) {
	t.Helper()
	if got.ID != scan.None || !got.Aggregate || got.Size != size || got.Count != count || got.FileCount != files {
		t.Errorf("%s = {ID:%d Aggregate:%v Size:%d Count:%d FileCount:%d}, want {ID:%d Aggregate:true Size:%d Count:%d FileCount:%d}",
			where, got.ID, got.Aggregate, got.Size, got.Count, got.FileCount, scan.None, size, count, files)
	}
	if got.Apparent != apparent {
		t.Errorf("%s.Apparent = %d, want %d", where, got.Apparent, apparent)
	}
}

func mustSubtree(t *testing.T, tree *scan.Tree, id scan.ID, depth, maxChildren int, minFraction float64) stats.View {
	t.Helper()
	v, ok := stats.Subtree(tree, id, depth, maxChildren, minFraction)
	if !ok {
		t.Fatalf("Subtree(%d, %d, %d, %v) ok = false", id, depth, maxChildren, minFraction)
	}
	return v
}

func TestSubtreeAggregate(t *testing.T) {
	tree, _ := fixture(t)
	v := mustSubtree(t, tree, tree.Root(), 1, 3, 0)

	checkView(t, "root", v, view(t, tree, tree.Root()))
	if len(v.Children) != 4 {
		t.Fatalf("len(root.Children) = %d, want 4 (x, b, pics, aggregate)", len(v.Children))
	}
	for i, name := range []string{"x", "b", "pics"} {
		checkView(t, fmt.Sprintf("Children[%d]", i), v.Children[i], view(t, tree, find(t, tree, name)))
	}
	// a 350 + docs 325 + r.txt 7 + empty 0 = 682; files 3+3+1+0 = 7.
	checkAggregate(t, "Children[3]", v.Children[3], 682*unit, 682*unit, 4, 7)
}

func TestSubtreeMinFraction(t *testing.T) {
	tree, _ := fixture(t)
	v := mustSubtree(t, tree, tree.Root(), 2, 0, 0.1)

	// Root threshold 719.2: keep x 5010, b 1000; merge pics 500 + a 350 + docs 325 + r.txt 7 + empty 0.
	if len(v.Children) != 3 {
		t.Fatalf("len(root.Children) = %d, want 3 (x, b, aggregate)", len(v.Children))
	}
	x, b := v.Children[0], v.Children[1]
	checkView(t, "x", x, view(t, tree, find(t, tree, "x")))
	checkView(t, "b", b, view(t, tree, find(t, tree, "b")))
	checkAggregate(t, "root aggregate", v.Children[2], 1182*unit, 1182*unit, 5, 9) // files 2+3+3+1+0

	// x threshold 501: keep y 5000; merge small.txt 10.
	if len(x.Children) != 2 {
		t.Fatalf("len(x.Children) = %d, want 2 (y, aggregate)", len(x.Children))
	}
	checkView(t, "x/y", x.Children[0], view(t, tree, find(t, tree, "x", "y")))
	checkAggregate(t, "x aggregate", x.Children[1], 10*unit, 10*unit, 1, 1)
	if len(x.Children[0].Children) != 0 {
		t.Errorf("x/y at depth 2 has %d children, want 0", len(x.Children[0].Children))
	}

	// b threshold 100: f4 1000 kept, nothing merged.
	if len(b.Children) != 1 {
		t.Fatalf("len(b.Children) = %d, want 1", len(b.Children))
	}
	checkView(t, "b/f4", b.Children[0], view(t, tree, find(t, tree, "b", "f4")))
}

func TestSubtreeDepth(t *testing.T) {
	tree, _ := fixture(t)
	root := view(t, tree, tree.Root())
	for _, d := range []int{0, -1} {
		v := mustSubtree(t, tree, tree.Root(), d, 0, 0)
		checkView(t, fmt.Sprintf("depth %d root", d), v, root)
		if len(v.Children) != 0 {
			t.Errorf("Subtree(depth=%d) has %d children, want 0", d, len(v.Children))
		}
	}

	v := mustSubtree(t, tree, tree.Root(), 1, 0, 0)
	n := mustNode(t, tree, tree.Root())
	if len(v.Children) != len(n.Children) {
		t.Fatalf("depth 1 unlimited: %d children, want %d", len(v.Children), len(n.Children))
	}
	for i, c := range v.Children {
		checkView(t, fmt.Sprintf("Children[%d]", i), c, view(t, tree, n.Children[i]))
		if len(c.Children) != 0 {
			t.Errorf("Children[%d] (%s) at depth 1 has %d children, want 0", i, c.Name, len(c.Children))
		}
	}
}

func TestSubtreeFileAndInvalid(t *testing.T) {
	tree, _ := fixture(t)
	id := find(t, tree, "x", "y", "z", "big.bin")
	v := mustSubtree(t, tree, id, 3, 0, 0)
	checkView(t, "big.bin", v, view(t, tree, id))
	if len(v.Children) != 0 {
		t.Errorf("file view has %d children, want 0", len(v.Children))
	}

	for _, bad := range []scan.ID{scan.None, scan.ID(1 << 30)} {
		if _, ok := stats.Subtree(tree, bad, 1, 0, 0); ok {
			t.Errorf("Subtree(%d) ok = true, want false", bad)
		}
	}
}

func TestSubtreeConservation(t *testing.T) {
	tree, _ := fixture(t)
	var walk func(where string, v stats.View, level, depth, maxChildren int)
	walk = func(where string, v stats.View, level, depth, maxChildren int) {
		if v.Aggregate || !v.IsDir || level >= depth {
			if len(v.Children) != 0 {
				t.Errorf("%s: unexpanded view has %d children", where, len(v.Children))
			}
			return
		}
		var sum, apparent, kept, merged int64
		for i, c := range v.Children {
			sum += c.Size
			apparent += c.Apparent
			if c.Aggregate {
				if i != len(v.Children)-1 {
					t.Errorf("%s: aggregate at index %d, want last", where, i)
				}
				merged = c.Count
			} else {
				kept++
			}
			walk(fmt.Sprintf("%s/%s", where, c.Name), c, level+1, depth, maxChildren)
		}
		if sum != v.Size {
			t.Errorf("%s: Σ children.Size = %d, want %d", where, sum, v.Size)
		}
		n := mustNode(t, tree, v.ID)
		if apparent != v.Apparent || v.Apparent != n.Apparent {
			t.Errorf("%s: Σ children.Apparent = %d, Apparent = %d, want both node Apparent %d", where, apparent, v.Apparent, n.Apparent)
		}
		if maxChildren > 0 && kept > int64(maxChildren) {
			t.Errorf("%s: %d non-aggregate children, max %d", where, kept, maxChildren)
		}
		if kept+merged != int64(len(n.Children)) {
			t.Errorf("%s: kept %d + merged %d != %d node children", where, kept, merged, len(n.Children))
		}
	}
	for depth := 0; depth <= 5; depth++ {
		for maxChildren := -1; maxChildren <= 4; maxChildren++ {
			for _, f := range []float64{-1, 0, 0.05, 0.1, 0.5, 1} {
				v := mustSubtree(t, tree, tree.Root(), depth, maxChildren, f)
				walk(fmt.Sprintf("[d=%d m=%d f=%v]", depth, maxChildren, f), v, 0, depth, maxChildren)
			}
		}
	}
}

func lastAggregate(t *testing.T, where string, v stats.View) stats.View {
	t.Helper()
	if len(v.Children) == 0 || !v.Children[len(v.Children)-1].Aggregate {
		t.Fatalf("%s: no trailing aggregate in %+v", where, v.Children)
	}
	return v.Children[len(v.Children)-1]
}

func checkNonAggregateSkipZero(t *testing.T, where string, v stats.View) {
	t.Helper()
	if !v.Aggregate && v.Skip != 0 {
		t.Errorf("%s (non-aggregate).Skip = %d, want 0", where, v.Skip)
	}
	for _, c := range v.Children {
		checkNonAggregateSkipZero(t, where+"/"+c.Name, c)
	}
}

func TestSubtreeAggregateSkip(t *testing.T) {
	tree, _ := fixture(t)

	v := mustSubtree(t, tree, tree.Root(), 1, 3, 0)
	if agg := lastAggregate(t, "maxChildren 3", v); agg.Skip != 3 || agg.Skip != int64(len(v.Children)-1) {
		t.Errorf("maxChildren 3: aggregate Skip = %d, want 3 (= %d kept children)", agg.Skip, len(v.Children)-1)
	}
	checkNonAggregateSkipZero(t, "maxChildren 3", v)

	v = mustSubtree(t, tree, tree.Root(), 1, 0, 0.1)
	if agg := lastAggregate(t, "minFraction 0.1", v); agg.Skip != 2 {
		t.Errorf("minFraction 0.1: aggregate Skip = %d, want 2 (x, b kept)", agg.Skip)
	}
	checkNonAggregateSkipZero(t, "minFraction 0.1", v)

	v = mustSubtree(t, tree, tree.Root(), 2, 1, 0)
	if agg := lastAggregate(t, "depth 2 root", v); agg.Skip != 1 {
		t.Errorf("depth 2 root aggregate Skip = %d, want 1", agg.Skip)
	}
	x := v.Children[0]
	checkView(t, "depth 2 Children[0]", x, view(t, tree, find(t, tree, "x")))
	if agg := lastAggregate(t, "depth 2 x", x); agg.Skip != 1 {
		t.Errorf("nested x aggregate Skip = %d, want 1", agg.Skip)
	}
	checkNonAggregateSkipZero(t, "depth 2", v)
}

func mustChildren(t *testing.T, tree *scan.Tree, id scan.ID, offset, limit int) []stats.Entry {
	t.Helper()
	es, ok := stats.Children(tree, id, offset, limit)
	if !ok {
		t.Fatalf("Children(%d, %d, %d) ok = false", id, offset, limit)
	}
	if es == nil {
		t.Fatalf("Children(%d, %d, %d) = nil, want non-nil slice", id, offset, limit)
	}
	return es
}

func entryIDs(es []stats.Entry) []scan.ID {
	ids := make([]scan.ID, len(es))
	for i, e := range es {
		ids[i] = e.ID
	}
	return ids
}

func entryNames(es []stats.Entry) []string {
	names := make([]string, len(es))
	for i, e := range es {
		names[i] = e.Name
	}
	return names
}

// checkChildrenMatch checks, for every expanded directory in v, that its kept children followed by
// Children(id, agg.Skip, agg.Count) are exactly Children(id, 0, 1000), and that the merged page sums to the aggregate.
func checkChildrenMatch(t *testing.T, tree *scan.Tree, where string, v stats.View) (aggregates int) {
	t.Helper()
	if v.Aggregate || len(v.Children) == 0 {
		return 0
	}
	var kept []scan.ID
	var merged []stats.Entry
	for _, c := range v.Children {
		if !c.Aggregate {
			kept = append(kept, c.ID)
			aggregates += checkChildrenMatch(t, tree, where+"/"+c.Name, c)
			continue
		}
		aggregates++
		merged = mustChildren(t, tree, v.ID, int(c.Skip), int(c.Count))
		if int64(len(merged)) != c.Count {
			t.Errorf("%s: Children(Skip %d, Count %d) returned %d entries, want %d", where, c.Skip, c.Count, len(merged), c.Count)
		}
		var size, apparent, nodeApparent, files int64
		for _, e := range merged {
			size += e.Size
			apparent += e.Apparent
			nodeApparent += mustNode(t, tree, e.ID).Apparent
			if e.IsDir {
				files += e.FileCount
			} else {
				files++
			}
			if slices.Contains(kept, e.ID) {
				t.Errorf("%s: merged child %q (ID %d) is also a kept child", where, e.Name, e.ID)
			}
		}
		if size != c.Size || files != c.FileCount {
			t.Errorf("%s: merged page Size %d FileCount %d, want aggregate Size %d FileCount %d", where, size, files, c.Size, c.FileCount)
		}
		if apparent != c.Apparent || c.Apparent != nodeApparent {
			t.Errorf("%s: merged page Apparent %d, aggregate Apparent %d, want both Σ node Apparent %d", where, apparent, c.Apparent, nodeApparent)
		}
	}
	all := entryIDs(mustChildren(t, tree, v.ID, 0, 1000))
	if got := append(kept, entryIDs(merged)...); !slices.Equal(got, all) {
		t.Errorf("%s: kept + merged IDs = %v, want Children(0, 1000) IDs %v", where, got, all)
	}
	return aggregates
}

func TestChildrenMatchesAggregate(t *testing.T) {
	tree, _ := fixture(t)
	for _, maxChildren := range []int{1, 2, 3} {
		where := fmt.Sprintf("[m=%d]", maxChildren)
		v := mustSubtree(t, tree, tree.Root(), 2, maxChildren, 0)
		if n := checkChildrenMatch(t, tree, where, v); n == 0 {
			t.Errorf("%s: no aggregate found in Subtree(root, 2, %d, 0)", where, maxChildren)
		}
	}
}

// nodeEntry builds the Entry Children should return for the node at names, given the RelPath.
func nodeEntry(t *testing.T, tree *scan.Tree, names ...string) stats.Entry {
	t.Helper()
	n := mustNode(t, tree, find(t, tree, names...))
	return stats.Entry{ID: n.ID, Name: n.Name, RelPath: filepath.Join(names...), Size: n.Size, Apparent: n.Apparent, IsDir: n.IsDir, FileCount: n.FileCount, ModTime: n.ModTime}
}

func TestChildrenOrderAndRelPath(t *testing.T) {
	tree, _ := fixture(t)

	root := mustChildren(t, tree, tree.Root(), 0, 100)
	if names := entryNames(root); !slices.Equal(names, []string{"x", "b", "pics", "a", "docs", "r.txt", "empty"}) {
		t.Fatalf("Children(root) names = %v, want [x b pics a docs r.txt empty]", names)
	}
	var want []stats.Entry
	for _, e := range root {
		if e.RelPath != e.Name {
			t.Errorf("Children(root) %q RelPath = %q, want %q", e.Name, e.RelPath, e.Name)
		}
		want = append(want, nodeEntry(t, tree, e.Name))
	}
	checkEntries(t, "Children(root)", root, want)

	a := mustChildren(t, tree, find(t, tree, "a"), 0, 100)
	checkEntries(t, "Children(a)", a, []stats.Entry{
		nodeEntry(t, tree, "a", "f2"),  // RelPath a/f2
		nodeEntry(t, tree, "a", "f1"),  // RelPath a/f1
		nodeEntry(t, tree, "a", "sub"), // RelPath a/sub
	})
	for _, e := range a {
		if want := "a" + string(filepath.Separator) + e.Name; e.RelPath != want {
			t.Errorf("Children(a) %q RelPath = %q, want %q", e.Name, e.RelPath, want)
		}
	}
	sub := mustChildren(t, tree, find(t, tree, "a", "sub"), 0, 100)
	checkEntries(t, "Children(a/sub)", sub, []stats.Entry{nodeEntry(t, tree, "a", "sub", "f3")})

	ranked := map[scan.ID]stats.Entry{}
	for _, e := range append(stats.TopFiles(tree, 1000), stats.TopDirs(tree, 1000)...) {
		ranked[e.ID] = e
	}
	compared := 0
	for _, es := range [][]stats.Entry{root, a, sub} {
		for _, e := range es {
			r, ok := ranked[e.ID]
			if !ok {
				continue
			}
			compared++
			if e != r {
				t.Errorf("Children entry %+v, want TopFiles/TopDirs entry %+v", e, r)
			}
		}
	}
	if compared == 0 {
		t.Error("no Children entry found in TopFiles/TopDirs to compare")
	}
}

func TestChildrenPaging(t *testing.T) {
	tree, _ := fixture(t)
	root := tree.Root()
	for _, c := range []struct {
		offset, limit int
		want          []string
	}{
		{0, 2, []string{"x", "b"}},
		{2, 2, []string{"pics", "a"}},
		{6, 5, []string{"empty"}},
		{7, 1, []string{}},
		{100, 1, []string{}},
		{0, 0, []string{}},
		{0, -1, []string{}},
		{-1, 3, []string{"x", "b", "pics"}},
	} {
		if got := entryNames(mustChildren(t, tree, root, c.offset, c.limit)); !slices.Equal(got, c.want) {
			t.Errorf("Children(root, %d, %d) names = %v, want %v", c.offset, c.limit, got, c.want)
		}
	}
	checkEntries(t, "Children(root, -1, 3) vs (0, 3)", mustChildren(t, tree, root, -1, 3), mustChildren(t, tree, root, 0, 3))

	for _, bad := range []scan.ID{scan.None, scan.ID(1 << 30)} {
		if got, ok := stats.Children(tree, bad, 0, 10); ok || got != nil {
			t.Errorf("Children(%d) = %+v, %v, want nil, false", bad, got, ok)
		}
	}
	if got, ok := stats.Children(nil, root, 0, 10); ok || got != nil {
		t.Errorf("Children(nil tree) = %+v, %v, want nil, false", got, ok)
	}
	if got := mustChildren(t, tree, find(t, tree, "r.txt"), 0, 10); len(got) != 0 {
		t.Errorf("Children(file) = %+v, want empty", got)
	}
}

func TestChildrenAfterRemove(t *testing.T) {
	tree, _ := fixture(t)
	pics := find(t, tree, "pics")
	if _, err := tree.Remove(pics); err != nil {
		t.Fatalf("Remove(pics): %v", err)
	}

	root := mustChildren(t, tree, tree.Root(), 0, 100)
	if names := entryNames(root); !slices.Equal(names, []string{"x", "b", "a", "docs", "r.txt", "empty"}) {
		t.Errorf("Children(root) after Remove(pics) names = %v, want [x b a docs r.txt empty]", names)
	}
	if got, ok := stats.Children(tree, pics, 0, 10); ok || got != nil {
		t.Errorf("Children(removed pics) = %+v, %v, want nil, false", got, ok)
	}

	v := mustSubtree(t, tree, tree.Root(), 1, 3, 0)
	if agg := lastAggregate(t, "after Remove", v); agg.Skip != 3 || agg.Count != 3 {
		t.Errorf("after Remove(pics): aggregate Skip %d Count %d, want 3, 3 (docs, r.txt, empty)", agg.Skip, agg.Count)
	}
	checkChildrenMatch(t, tree, "after Remove", v)
}

func TestCrumbs(t *testing.T) {
	tree, root := fixture(t)
	abs, err := filepath.Abs(root)
	if err != nil {
		t.Fatal(err)
	}

	want := []stats.Crumb{{ID: tree.Root(), Name: filepath.Clean(abs)}}
	if got := stats.Crumbs(tree, tree.Root()); !slices.Equal(got, want) {
		t.Errorf("Crumbs(root) = %+v, want %+v", got, want)
	}

	path := []string{"x", "y", "z", "big.bin"}
	for i := range path {
		want = append(want, stats.Crumb{ID: find(t, tree, path[:i+1]...), Name: path[i]})
	}
	if got := stats.Crumbs(tree, want[len(want)-1].ID); !slices.Equal(got, want) {
		t.Errorf("Crumbs(big.bin) = %+v, want %+v", got, want)
	}

	for _, bad := range []scan.ID{scan.None, scan.ID(1 << 30)} {
		if got := stats.Crumbs(tree, bad); len(got) != 0 {
			t.Errorf("Crumbs(%d) = %+v, want empty", bad, got)
		}
	}
}

// collectCall is one keep call seen by collectRecorder: its arguments and result.
type collectCall struct {
	path string
	node scan.Node
	kept bool
}

// collectRecorder wraps keep and records every call to it.
type collectRecorder struct {
	keep  func(string, scan.Node) bool
	calls []collectCall
}

func (r *collectRecorder) Keep(path string, n scan.Node) bool {
	kept := r.keep(path, n)
	r.calls = append(r.calls, collectCall{path, n, kept})
	return kept
}

// collectNames keeps the nodes whose Name is one of names.
func collectNames(names ...string) func(string, scan.Node) bool {
	return func(_ string, n scan.Node) bool { return slices.Contains(names, n.Name) }
}

func collectFiles(_ string, n scan.Node) bool { return !n.IsDir }

func collectAlways(string, scan.Node) bool { return true }

func collectNever(string, scan.Node) bool { return false }

func collectCheck(t *testing.T, where string, c stats.Collection, want []stats.Entry, count, total int64) {
	t.Helper()
	checkEntries(t, where+".Entries", c.Entries, want)
	if c.Count != count || c.Total != total {
		t.Errorf("%s Count/Total = %d/%d, want %d/%d", where, c.Count, c.Total, count, total)
	}
}

func collectEmpty(t *testing.T, where string, c stats.Collection) {
	t.Helper()
	if c.Entries == nil || len(c.Entries) != 0 || c.Total != 0 || c.Count != 0 {
		t.Errorf("%s = {Entries:%+v (nil %v) Total:%d Count:%d}, want non-nil empty Entries, Total 0, Count 0",
			where, c.Entries, c.Entries == nil, c.Total, c.Count)
	}
}

func TestCollectByName(t *testing.T) {
	tree, _ := fixture(t)
	c := stats.Collect(tree, 0, collectNames("z", "sub", "b.jpg", "noext"))
	collectCheck(t, "Collect(names z sub b.jpg noext)", c, []stats.Entry{
		entry(t, tree, "x", "y", "z"),   // 5000
		entry(t, tree, "pics", "b.jpg"), // 100
		entry(t, tree, "a", "sub"),      // 50
		entry(t, tree, "docs", "noext"), // 20
	}, 4, 5170*unit)
}

func TestCollectTopmost(t *testing.T) {
	tree, _ := fixture(t)
	z := find(t, tree, "x", "y", "z")
	big := find(t, tree, "x", "y", "z", "big.bin")
	below := []string{tree.Path(z), tree.Path(big)}
	rec := &collectRecorder{keep: collectNames("y", "z")}
	collectCheck(t, "Collect(names y z)", stats.Collect(tree, 0, rec.Keep), []stats.Entry{entry(t, tree, "x", "y")}, 1, 5000*unit)
	for _, call := range rec.calls {
		if call.node.ID == z || call.node.ID == big || slices.Contains(below, call.path) {
			t.Errorf("Collect(names y z) called keep(%q) on ID %d inside the hit x/y", call.path, call.node.ID)
		}
	}
}

func TestCollectByPathPrefix(t *testing.T) {
	tree, _ := fixture(t)
	y := tree.Path(find(t, tree, "x", "y"))
	c := stats.Collect(tree, 0, func(path string, _ scan.Node) bool { return strings.HasPrefix(path, y) })
	collectCheck(t, fmt.Sprintf("Collect(path prefix %q)", y), c, []stats.Entry{entry(t, tree, "x", "y")}, 1, 5000*unit)
}

func TestCollectMatchesTopFiles(t *testing.T) {
	tree, _ := fixture(t)
	for _, c := range []struct{ limit, n int }{{0, 100}, {3, 3}} {
		got := stats.Collect(tree, c.limit, collectFiles)
		collectCheck(t, fmt.Sprintf("Collect(%d, files) vs TopFiles(%d)", c.limit, c.n), got, stats.TopFiles(tree, c.n), 12, 7192*unit)
	}
}

func TestCollectLimit(t *testing.T) {
	tree, _ := fixture(t)
	all := stats.TopFiles(tree, 100)
	for _, c := range []struct{ limit, want int }{{1, 1}, {12, 12}, {100, 12}, {0, 12}, {-1, 12}} {
		got := stats.Collect(tree, c.limit, collectFiles)
		collectCheck(t, fmt.Sprintf("Collect(%d, files)", c.limit), got, all[:c.want], 12, 7192*unit)
	}
}

func TestCollectNever(t *testing.T) {
	tree, _ := fixture(t)
	rec := &collectRecorder{keep: collectNever}
	collectEmpty(t, "Collect(never)", stats.Collect(tree, 0, rec.Keep))
	if len(rec.calls) != tree.Len() {
		t.Errorf("Collect(never) called keep %d times, want Tree.Len() = %d", len(rec.calls), tree.Len())
	}
}

func TestCollectRoot(t *testing.T) {
	tree, root := fixture(t)
	abs, err := filepath.Abs(root)
	if err != nil {
		t.Fatal(err)
	}
	n := mustNode(t, tree, tree.Root())
	want := stats.Entry{ID: tree.Root(), Name: filepath.Clean(abs), RelPath: ".", Size: 7192 * unit, Apparent: n.Apparent, IsDir: true, FileCount: 12, ModTime: n.ModTime}
	rec := &collectRecorder{keep: collectAlways}
	collectCheck(t, "Collect(always)", stats.Collect(tree, 0, rec.Keep), []stats.Entry{want}, 1, 7192*unit)
	if len(rec.calls) != 1 {
		t.Errorf("Collect(always) called keep %d times, want 1 (the root)", len(rec.calls))
	}
}

func TestCollectPathEqualsTreePath(t *testing.T) {
	tree, _ := fixture(t)
	rec := &collectRecorder{keep: collectNever}
	stats.Collect(tree, 0, rec.Keep)
	seen := map[scan.ID]bool{}
	for _, call := range rec.calls {
		id := call.node.ID
		if want := tree.Path(id); call.path != want {
			t.Errorf("Collect keep path for ID %d = %q, want Tree.Path %q", id, call.path, want)
		}
		if want, _ := tree.Node(id); !reflect.DeepEqual(call.node, want) {
			t.Errorf("Collect keep node = %+v, want Tree.Node(%d) = %+v", call.node, id, want)
		}
		if seen[id] {
			t.Errorf("Collect called keep twice for ID %d (%q)", id, call.path)
		}
		seen[id] = true
	}
	if len(rec.calls) != tree.Len() {
		t.Errorf("Collect(never) called keep %d times, want Tree.Len() = %d", len(rec.calls), tree.Len())
	}
}

func TestCollectAfterRemove(t *testing.T) {
	tree, _ := fixture(t)
	pics := find(t, tree, "pics")
	picsPath := tree.Path(pics)
	keep := collectNames("pics", "b", "a.JPG")
	// a.JPG lies inside the hit pics, so it is never visited.
	collectCheck(t, "Collect(names pics b a.JPG)", stats.Collect(tree, 0, keep),
		[]stats.Entry{entry(t, tree, "b"), entry(t, tree, "pics")}, 2, 1500*unit)

	if _, err := tree.Remove(pics); err != nil {
		t.Fatalf("Remove(pics): %v", err)
	}
	rec := &collectRecorder{keep: keep}
	collectCheck(t, "Collect(names pics b a.JPG) after Remove(pics)", stats.Collect(tree, 0, rec.Keep),
		[]stats.Entry{entry(t, tree, "b")}, 1, 1000*unit)
	for _, call := range rec.calls {
		if strings.HasPrefix(call.path, picsPath) {
			t.Errorf("Collect after Remove(pics) called keep(%q)", call.path)
		}
	}
}

func TestCollectInvariants(t *testing.T) {
	tree, _ := fixture(t)
	y := tree.Path(find(t, tree, "x", "y"))
	pics := find(t, tree, "pics")
	keeps := []struct {
		name string
		keep func(string, scan.Node) bool
	}{
		{"names z sub b.jpg noext", collectNames("z", "sub", "b.jpg", "noext")},
		{"names y z", collectNames("y", "z")},
		{"path prefix x/y", func(path string, _ scan.Node) bool { return strings.HasPrefix(path, y) }},
		{"files", collectFiles},
		{"never", collectNever},
		{"always", collectAlways},
		{"names pics b a.JPG", collectNames("pics", "b", "a.JPG")},
	}
	for _, removed := range []bool{false, true} {
		if removed {
			if _, err := tree.Remove(pics); err != nil {
				t.Fatalf("Remove(pics): %v", err)
			}
		}
		for _, k := range keeps {
			rec := &collectRecorder{keep: k.keep}
			c := stats.Collect(tree, 0, rec.Keep)
			collectInvariants(t, tree, fmt.Sprintf("Collect(%s) [pics removed %v]", k.name, removed), c, rec.calls)
		}
	}
}

// collectInvariants checks c = Collect(tree, 0, keep) against keep's recorded calls: Count and Total sum the hits,
// Entries are exactly the hits, none is an ancestor of another, and they are sorted by Size desc, RelPath asc.
func collectInvariants(t *testing.T, tree *scan.Tree, where string, c stats.Collection, calls []collectCall) {
	t.Helper()
	if len(calls) == 0 {
		t.Errorf("%s called keep 0 times, want at least once (the root)", where)
		return
	}
	var hits []scan.ID
	var total int64
	for _, call := range calls {
		if call.kept {
			hits = append(hits, call.node.ID)
			total += call.node.Size
		}
	}
	if c.Count != int64(len(hits)) || c.Total != total {
		t.Errorf("%s Count/Total = %d/%d, want %d/%d (keep hits)", where, c.Count, c.Total, len(hits), total)
	}
	ids := entryIDs(c.Entries)
	slices.Sort(ids)
	slices.Sort(hits)
	if !slices.Equal(ids, hits) {
		t.Errorf("%s Entries IDs = %v, want keep hits %v", where, ids, hits)
	}
	for _, e := range c.Entries {
		for p := mustNode(t, tree, e.ID).Parent; p != scan.None; p = mustNode(t, tree, p).Parent {
			if slices.Contains(ids, p) {
				t.Errorf("%s entry %q has its ancestor ID %d among the Entries", where, e.RelPath, p)
			}
		}
	}
	if !slices.IsSortedFunc(c.Entries, func(a, b stats.Entry) int {
		return cmp.Or(cmp.Compare(b.Size, a.Size), strings.Compare(a.RelPath, b.RelPath))
	}) {
		t.Errorf("%s Entries not sorted by Size desc, RelPath asc: %+v", where, c.Entries)
	}
}

func TestCollectNil(t *testing.T) {
	tree, _ := fixture(t)
	collectEmpty(t, "Collect(nil tree)", stats.Collect(nil, 0, collectAlways))
	collectEmpty(t, "Collect(nil keep)", stats.Collect(tree, 0, nil))
}
