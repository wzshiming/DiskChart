package scan

import (
	"path/filepath"
	"slices"
	"strings"
	"sync"
)

// Tree is the result of Scan. Read methods are safe for concurrent use.
type Tree struct {
	mu    sync.RWMutex
	nodes []node // parents always precede their children
	root  ID
	errs  []Error
	live  int
}

type node struct {
	name      string
	children  []ID
	size      int64
	apparent  int64
	fileCount int64
	dirCount  int64
	modTime   int64
	parent    ID
	flags     Flags
	isDir     bool
	removed   bool
}

// finish aggregates sizes bottom-up and sorts every directory's children.
func (t *Tree) finish() {
	n := t.nodes
	for i := len(n) - 1; i > 0; i-- {
		c := &n[i]
		p := &n[c.parent]
		p.size += c.size
		p.apparent += c.apparent
		if c.isDir {
			p.fileCount += c.fileCount
			p.dirCount += 1 + c.dirCount
		} else {
			p.fileCount++
		}
	}
	for i := range n {
		if len(n[i].children) > 1 {
			slices.SortFunc(n[i].children, t.cmp)
		}
	}
	t.live = len(n)
}

// cmp orders by Size descending, then Name ascending.
func (t *Tree) cmp(a, b ID) int {
	na, nb := &t.nodes[a], &t.nodes[b]
	if na.size != nb.size {
		if na.size > nb.size {
			return -1
		}
		return 1
	}
	return strings.Compare(na.name, nb.name)
}

func (t *Tree) valid(id ID) bool {
	return id >= 0 && int(id) < len(t.nodes) && !t.nodes[id].removed
}

// Root returns the ID of the root node.
func (t *Tree) Root() ID { return t.root }

// Len returns the number of live nodes.
func (t *Tree) Len() int {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.live
}

// Node returns the node with the given ID, or false if it is invalid or removed.
func (t *Tree) Node(id ID) (Node, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	if !t.valid(id) {
		return Node{}, false
	}
	n := &t.nodes[id]
	return Node{
		ID:        id,
		Parent:    n.parent,
		Name:      n.name,
		IsDir:     n.isDir,
		Size:      n.size,
		Apparent:  n.apparent,
		FileCount: n.fileCount,
		DirCount:  n.dirCount,
		ModTime:   n.modTime,
		Flags:     n.flags,
		Children:  n.children,
	}, true
}

// Path returns the absolute path of the node, or "" if it is invalid.
func (t *Tree) Path(id ID) string {
	t.mu.RLock()
	defer t.mu.RUnlock()
	if !t.valid(id) {
		return ""
	}
	var names []string
	for x := id; x != None; x = t.nodes[x].parent {
		names = append(names, t.nodes[x].name)
	}
	slices.Reverse(names)
	return filepath.Join(names...)
}

// Errors returns the read errors in the order they were encountered.
func (t *Tree) Errors() []Error {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return slices.Clone(t.errs)
}

// Remove detaches the subtree at id, updates ancestor totals, and returns its parent.
func (t *Tree) Remove(id ID) (parent ID, err error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if !t.valid(id) {
		return None, ErrInvalidID
	}
	if id == t.root {
		return None, ErrRoot
	}
	n := &t.nodes[id]
	size, apparent, files, dirs := n.size, n.apparent, int64(1), int64(0)
	if n.isDir {
		files, dirs = n.fileCount, 1+n.dirCount
	}
	parent = n.parent

	stack := []ID{id}
	for len(stack) > 0 {
		x := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		m := &t.nodes[x]
		stack = append(stack, m.children...)
		m.removed = true
		m.children = nil
		t.live--
	}

	// Slices handed out by Node are never mutated; replace them instead.
	p := &t.nodes[parent]
	kids := make([]ID, 0, len(p.children)-1)
	for _, c := range p.children {
		if c != id {
			kids = append(kids, c)
		}
	}
	p.children = kids

	for a := parent; a != None; a = t.nodes[a].parent {
		an := &t.nodes[a]
		an.size -= size
		an.apparent -= apparent
		an.fileCount -= files
		an.dirCount -= dirs
		if g := an.parent; g != None {
			t.nodes[g].children = t.reposition(t.nodes[g].children, a)
		}
	}
	return parent, nil
}

// reposition moves a, whose size just shrank, to its sorted place in kids.
func (t *Tree) reposition(kids []ID, a ID) []ID {
	i := slices.Index(kids, a)
	j := i
	for j+1 < len(kids) && t.cmp(kids[j+1], a) < 0 {
		j++
	}
	if j == i {
		return kids
	}
	out := make([]ID, 0, len(kids))
	out = append(out, kids[:i]...)
	out = append(out, kids[i+1:j+1]...)
	out = append(out, a)
	out = append(out, kids[j+1:]...)
	return out
}
