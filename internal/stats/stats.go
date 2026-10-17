// Package stats answers size questions over a scan.Tree: top files and directories, extension breakdowns, and bounded subtree views.
package stats

import (
	"cmp"
	"path/filepath"
	"slices"
	"strings"

	"github.com/wzshiming/DiskChart/internal/scan"
)

// DominanceThreshold is the fraction of a directory's size above which a single child directory hides it from TopDirs.
const DominanceThreshold = 0.9

// Entry is one ranked file or directory.
type Entry struct {
	ID        scan.ID
	Name      string
	RelPath   string // filepath.Rel(root path, node path)
	Size      int64
	IsDir     bool
	FileCount int64
	ModTime   int64
}

// View is a depth- and width-bounded snapshot of a subtree.
type View struct {
	ID        scan.ID // scan.None for the aggregate pseudo-node
	Name      string
	Size      int64
	IsDir     bool
	FileCount int64
	Aggregate bool
	Count     int64 // aggregate pseudo-node only: number of merged children
	Skip      int64 // aggregate pseudo-node only: index of the first merged child among the parent's live children in display order (== number of kept children)
	Children  []View
}

// Crumb is one step on the path from the root to a node.
type Crumb struct {
	ID   scan.ID
	Name string
}

type cand struct {
	id       scan.ID
	size     int64
	prel     string // parent's relative path
	name     string
	maxChild int64 // largest child directory size (TopDirs only)
}

// walk visits every live node reachable from the root, passing the parent's relative path.
func walk(t *scan.Tree, fn func(n scan.Node, prel string)) {
	type item struct {
		id   scan.ID
		prel string
	}
	root := t.Root()
	stack := []item{{root, ""}}
	for len(stack) > 0 {
		it := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		n, ok := t.Node(it.id)
		if !ok {
			continue
		}
		fn(n, it.prel)
		if len(n.Children) == 0 {
			continue
		}
		rel := "."
		if n.ID != root {
			rel = joinRel(it.prel, n.Name)
		}
		for _, v := range slices.Backward(n.Children) {
			stack = append(stack, item{v, rel})
		}
	}
}

func joinRel(prel, name string) string {
	if prel == "." || prel == "" {
		return name
	}
	return prel + string(filepath.Separator) + name
}

// TopFiles returns the n largest non-directory nodes, by Size desc then RelPath asc.
func TopFiles(t *scan.Tree, n int) []Entry {
	if t == nil || n <= 0 {
		return []Entry{}
	}
	var cs []cand
	walk(t, func(x scan.Node, prel string) {
		if !x.IsDir {
			cs = append(cs, cand{id: x.ID, size: x.Size, prel: prel, name: x.Name})
		}
	})
	return top(t, cs, n)
}

// TopDirs returns the n largest non-root, non-empty directories not dominated by a single child directory.
func TopDirs(t *scan.Tree, n int) []Entry {
	if t == nil || n <= 0 {
		return []Entry{}
	}
	root := t.Root()
	var cs []cand
	var idx []int32 // node ID -> index in cs, or -1
	walk(t, func(x scan.Node, prel string) {
		if !x.IsDir || x.ID == root {
			return
		}
		for int(x.ID) >= len(idx) {
			idx = append(idx, -1)
		}
		idx[x.ID] = int32(len(cs))
		cs = append(cs, cand{id: x.ID, size: x.Size, prel: prel, name: x.Name})
		if int(x.Parent) < len(idx) && x.Parent >= 0 {
			if p := idx[x.Parent]; p >= 0 && x.Size > cs[p].maxChild {
				cs[p].maxChild = x.Size
			}
		}
	})
	kept := cs[:0]
	for _, c := range cs {
		if c.size > 0 && float64(c.maxChild) < DominanceThreshold*float64(c.size) {
			kept = append(kept, c)
		}
	}
	return top(t, kept, n)
}

// top sorts candidates by size desc, rel path asc, and returns the first n as entries.
func top(t *scan.Tree, cs []cand, n int) []Entry {
	if len(cs) > n {
		thr := nthLargest(cs, n)
		kept := cs[:0]
		for _, c := range cs {
			if c.size >= thr {
				kept = append(kept, c)
			}
		}
		cs = kept
	}
	type ranked struct {
		id   scan.ID
		size int64
		rel  string
	}
	rs := make([]ranked, len(cs))
	for i, c := range cs {
		rs[i] = ranked{c.id, c.size, joinRel(c.prel, c.name)}
	}
	slices.SortFunc(rs, func(a, b ranked) int {
		if c := cmp.Compare(b.size, a.size); c != 0 {
			return c
		}
		return strings.Compare(a.rel, b.rel)
	})
	if len(rs) > n {
		rs = rs[:n]
	}
	out := make([]Entry, 0, len(rs))
	for _, r := range rs {
		x, ok := t.Node(r.id)
		if !ok {
			continue
		}
		out = append(out, Entry{
			ID:        x.ID,
			Name:      x.Name,
			RelPath:   r.rel,
			Size:      x.Size,
			IsDir:     x.IsDir,
			FileCount: x.FileCount,
			ModTime:   x.ModTime,
		})
	}
	return out
}

// nthLargest returns the n-th largest size in cs using a size-n min-heap; requires len(cs) > n > 0.
func nthLargest(cs []cand, n int) int64 {
	h := make([]int64, 0, n)
	down := func(i int) {
		for {
			l := 2*i + 1
			if l >= len(h) {
				return
			}
			m := l
			if r := l + 1; r < len(h) && h[r] < h[l] {
				m = r
			}
			if h[i] <= h[m] {
				return
			}
			h[i], h[m] = h[m], h[i]
			i = m
		}
	}
	for _, c := range cs {
		if len(h) < n {
			h = append(h, c.size)
			for i := len(h) - 1; i > 0; {
				p := (i - 1) / 2
				if h[p] <= h[i] {
					break
				}
				h[p], h[i] = h[i], h[p]
				i = p
			}
		} else if c.size > h[0] {
			h[0] = c.size
			down(0)
		}
	}
	return h[0]
}

// Subtree returns a view of id expanded depth levels, keeping at most maxChildren children of at least minFraction of their parent and merging the rest into a trailing aggregate.
func Subtree(t *scan.Tree, id scan.ID, depth, maxChildren int, minFraction float64) (View, bool) {
	if t == nil {
		return View{}, false
	}
	n, ok := t.Node(id)
	if !ok {
		return View{}, false
	}
	return subtree(t, n, depth, maxChildren, minFraction), true
}

func subtree(t *scan.Tree, n scan.Node, depth, maxChildren int, minFraction float64) View {
	v := View{ID: n.ID, Name: n.Name, Size: n.Size, IsDir: n.IsDir, FileCount: n.FileCount}
	if depth <= 0 || !n.IsDir {
		return v
	}
	agg := View{ID: scan.None, Aggregate: true}
	selected := 0
	for _, cid := range n.Children {
		c, ok := t.Node(cid)
		if !ok {
			continue
		}
		if (maxChildren <= 0 || selected < maxChildren) &&
			(n.Size == 0 || minFraction <= 0 || float64(c.Size) >= minFraction*float64(n.Size)) {
			v.Children = append(v.Children, subtree(t, c, depth-1, maxChildren, minFraction))
			selected++
			continue
		}
		agg.Size += c.Size
		agg.Count++
		if c.IsDir {
			agg.FileCount += c.FileCount
		} else {
			agg.FileCount++
		}
	}
	if agg.Count > 0 {
		agg.Skip = int64(selected)
		v.Children = append(v.Children, agg)
	}
	return v
}

// Children returns up to limit live children of id in display order starting at offset; ok is false if id is invalid.
func Children(t *scan.Tree, id scan.ID, offset, limit int) ([]Entry, bool) {
	if t == nil {
		return nil, false
	}
	n, ok := t.Node(id)
	if !ok {
		return nil, false
	}
	out := []Entry{}
	if !n.IsDir || limit <= 0 {
		return out, true
	}
	offset = max(offset, 0)
	prel := relPath(t, n)
	i := 0
	for _, cid := range n.Children {
		c, ok := t.Node(cid)
		if !ok {
			continue
		}
		if i++; i <= offset {
			continue
		}
		out = append(out, Entry{
			ID:        c.ID,
			Name:      c.Name,
			RelPath:   joinRel(prel, c.Name),
			Size:      c.Size,
			IsDir:     c.IsDir,
			FileCount: c.FileCount,
			ModTime:   c.ModTime,
		})
		if len(out) == limit {
			break
		}
	}
	return out, true
}

// relPath returns n's path relative to the root, "." for the root itself.
func relPath(t *scan.Tree, n scan.Node) string {
	root := t.Root()
	var names []string
	for n.ID != root {
		names = append(names, n.Name)
		p, ok := t.Node(n.Parent)
		if !ok {
			break
		}
		n = p
	}
	rel := "."
	for _, name := range slices.Backward(names) {
		rel = joinRel(rel, name)
	}
	return rel
}

// Crumbs returns the path from the root to id, or nil if id is invalid.
func Crumbs(t *scan.Tree, id scan.ID) []Crumb {
	if t == nil {
		return nil
	}
	var out []Crumb
	for x := id; x != scan.None; {
		n, ok := t.Node(x)
		if !ok {
			return nil
		}
		out = append(out, Crumb{ID: n.ID, Name: n.Name})
		x = n.Parent
	}
	slices.Reverse(out)
	return out
}
