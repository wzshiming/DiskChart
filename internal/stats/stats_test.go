package stats_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/wzshiming/diskchart/internal/scan"
	"github.com/wzshiming/diskchart/internal/stats"
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

// fixture builds (sizes in bytes, total 7192):
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
	e := stats.Entry{ID: id, Name: n.Name, RelPath: filepath.Join(names...), Size: n.Size, IsDir: n.IsDir, ModTime: n.ModTime}
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
	ext("bin", 5000, 1),
	ext("", 1375, 6),
	ext("jpg", 500, 2),
	ext("pdf", 300, 1),
	ext("txt", 17, 2),
}

func TestByExtension(t *testing.T) {
	tree, _ := fixture(t)
	got := stats.ByExtension(tree, 2)

	if !slices.Equal(got.Items, allExts[:2]) {
		t.Errorf("ByExtension(2).Items:\n got  %+v\n want %+v", got.Items, allExts[:2])
	}
	// Other = jpg+pdf+txt = 500+300+17 = 817, Count 2+1+2 = 5.
	wantOther := stats.ExtStat{Ext: "", Category: "other", Size: 817, Count: 5}
	if got.Other != wantOther {
		t.Errorf("ByExtension(2).Other = %+v, want %+v", got.Other, wantOther)
	}
	if got.Total != 7192 {
		t.Errorf("ByExtension(2).Total = %d, want 7192", got.Total)
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
		if got.Total != 7192 {
			t.Errorf("ByExtension(%d).Total = %d, want 7192", k, got.Total)
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
	return stats.View{ID: n.ID, Name: n.Name, Size: n.Size, IsDir: n.IsDir, FileCount: n.FileCount}
}

// checkView compares the node fields of got against want, ignoring Children.
func checkView(t *testing.T, where string, got, want stats.View) {
	t.Helper()
	if got.ID != want.ID || got.Name != want.Name || got.Size != want.Size || got.IsDir != want.IsDir || got.Aggregate != want.Aggregate {
		t.Errorf("%s = {ID:%d Name:%q Size:%d IsDir:%v Aggregate:%v}, want {ID:%d Name:%q Size:%d IsDir:%v Aggregate:%v}",
			where, got.ID, got.Name, got.Size, got.IsDir, got.Aggregate, want.ID, want.Name, want.Size, want.IsDir, want.Aggregate)
	}
	if want.IsDir && got.FileCount != want.FileCount {
		t.Errorf("%s.FileCount = %d, want %d", where, got.FileCount, want.FileCount)
	}
}

func checkAggregate(t *testing.T, where string, got stats.View, size, count, files int64) {
	t.Helper()
	if got.ID != scan.None || !got.Aggregate || got.Size != size || got.Count != count || got.FileCount != files {
		t.Errorf("%s = {ID:%d Aggregate:%v Size:%d Count:%d FileCount:%d}, want {ID:%d Aggregate:true Size:%d Count:%d FileCount:%d}",
			where, got.ID, got.Aggregate, got.Size, got.Count, got.FileCount, scan.None, size, count, files)
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
	checkAggregate(t, "Children[3]", v.Children[3], 682, 4, 7)
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
	checkAggregate(t, "root aggregate", v.Children[2], 1182, 5, 9) // files 2+3+3+1+0

	// x threshold 501: keep y 5000; merge small.txt 10.
	if len(x.Children) != 2 {
		t.Fatalf("len(x.Children) = %d, want 2 (y, aggregate)", len(x.Children))
	}
	checkView(t, "x/y", x.Children[0], view(t, tree, find(t, tree, "x", "y")))
	checkAggregate(t, "x aggregate", x.Children[1], 10, 1, 1)
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
		var sum, kept, merged int64
		for i, c := range v.Children {
			sum += c.Size
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
		if maxChildren > 0 && kept > int64(maxChildren) {
			t.Errorf("%s: %d non-aggregate children, max %d", where, kept, maxChildren)
		}
		if n := mustNode(t, tree, v.ID); kept+merged != int64(len(n.Children)) {
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
