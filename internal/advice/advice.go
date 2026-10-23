// Package advice classifies absolute paths as protected from deletion, recommended for cleanup, or neither, and says why.
package advice

import (
	"os"
	"path/filepath"
	"strings"
)

// Class is the deletion advice for a path.
type Class string

// Deletion advice classes.
const (
	None        Class = ""
	Protected   Class = "protected"
	Recommended Class = "recommended"
)

// Reason explains a Class.
type Reason string

// Reasons; Root is assigned to the scan root by the caller and never returned by Classify.
const (
	Root        Reason = "root"
	Volume      Reason = "volume"
	Home        Reason = "home"
	System      Reason = "system"
	UserData    Reason = "userdata"
	Contains    Reason = "contains"
	Cache       Reason = "cache"
	Temp        Reason = "temp"
	Trash       Reason = "trash"
	Logs        Reason = "logs"
	Rebuildable Reason = "rebuildable"
	Custom      Reason = "custom"
)

// Verdict is the advice for one path; Class is None exactly when Reason is empty.
type Verdict struct {
	Class  Class
	Reason Reason
}

// UserRules are user-configured rules; paths may start with ~ to mean Home.
type UserRules struct {
	Protected   []string // subtrees that must never be deleted
	Recommended []string // directories whose contents are safe to clean up
	Names       []string // directory base names safe to clean up wherever they occur
}

// Config describes the host the rules are compiled for; non-absolute paths are ignored.
type Config struct {
	// GOOS selects the platform table and should equal the host's runtime.GOOS: path syntax follows
	// the host's filepath, so absolute-path checks are unreliable when compiling another GOOS's table.
	// darwin and windows compare case-insensitively.
	GOOS        string
	Home        string              // home directory; "" drops every ~ anchor
	TempDir     string              // temporary directory
	UID         string              // user id text (numeric on Unix, SID on Windows); "" drops per-volume trash anchors
	Mountpoints []string            // mount points, protected as volume roots
	Getenv      func(string) string // environment lookup; nil means os.Getenv
	User        UserRules
}

// Rules is an immutable set of compiled advice rules, safe for concurrent use.
type Rules struct {
	fold  bool // compare path segments case-insensitively
	root  *node
	names map[string]Reason
}

// New compiles the rules for c; it performs no I/O and keeps no reference to c.
func New(c Config) *Rules {
	b := &builder{
		fold:   c.GOOS == "darwin" || c.GOOS == "windows",
		root:   &node{},
		names:  map[string]Reason{},
		goos:   c.GOOS,
		home:   abs(c.Home),
		tmp:    abs(c.TempDir),
		uid:    c.UID,
		getenv: c.Getenv,
	}
	if b.getenv == nil {
		b.getenv = os.Getenv
	}
	for _, m := range c.Mountpoints {
		if m = abs(m); m != "" {
			b.mounts = append(b.mounts, m)
		}
	}
	if b.home != "" {
		b.mounts = append(b.mounts, filepath.VolumeName(b.home)+string(filepath.Separator))
	}
	b.common()
	switch c.GOOS {
	case "darwin":
		b.darwin()
	case "linux":
		b.linux()
	case "windows":
		b.windows()
	}
	b.user(c.User)
	b.root.finish()
	return &Rules{fold: b.fold, root: b.root, names: b.names}
}

// Classify returns the advice for path, which must be absolute; dir says whether path is a directory.
func (r *Rules) Classify(path string, dir bool) Verdict {
	p := filepath.Clean(path)
	if !filepath.IsAbs(p) {
		return Verdict{}
	}
	if r.fold {
		p = strings.ToLower(p)
	}
	vol := filepath.VolumeName(p)
	rest := p[len(vol):]
	if len(rest) <= 1 { // a lone separator, or nothing after a UNC share: a volume root
		return Verdict{Class: Protected, Reason: Volume}
	}
	rest = rest[1:]
	n := r.root.children[vol]
	var cover Verdict // deepest subtree or content anchor strictly above p
	var hit Reason    // first segment matching a name rule
	for {
		if n != nil {
			switch {
			case n.subtree != "":
				cover = Verdict{Class: Protected, Reason: n.subtree}
			case n.content != "":
				cover = Verdict{Class: Recommended, Reason: n.content}
			}
		}
		seg, last := rest, true
		if i := strings.IndexByte(rest, filepath.Separator); i >= 0 {
			seg, rest, last = rest[:i], rest[i+1:], false
		}
		if n != nil {
			n = n.children[seg]
		}
		if hit == "" && (dir || !last) {
			hit = r.names[seg]
		}
		if last {
			break
		}
	}
	switch {
	case n != nil && n.self != "":
		return Verdict{Class: Protected, Reason: n.self}
	case n != nil && n.below:
		return Verdict{Class: Protected, Reason: Contains}
	case cover.Class != None:
		return cover
	case hit != "":
		return Verdict{Class: Recommended, Reason: hit}
	}
	return Verdict{}
}

// kind is how an anchor path applies its advice.
type kind uint8

const (
	point   kind = iota // the anchor itself is protected; its contents are unaffected
	subtree             // the anchor and everything beneath it are protected
	content             // everything beneath the anchor is recommended; the anchor itself is not
)

// node is one path segment of the anchor trie; the first level holds volume names.
type node struct {
	children map[string]*node
	self     Reason // highest-ranked point or subtree reason anchored here
	subtree  Reason // highest-ranked subtree reason anchored here
	content  Reason // first content reason anchored here
	below    bool   // a point or subtree anchor lies strictly beneath
}

func (n *node) child(seg string) *node {
	c := n.children[seg]
	if c == nil {
		if n.children == nil {
			n.children = map[string]*node{}
		}
		c = &node{}
		n.children[seg] = c
	}
	return c
}

// finish computes below for the subtree and reports whether n or any descendant is a protected anchor.
func (n *node) finish() bool {
	for _, c := range n.children {
		if c.finish() {
			n.below = true
		}
	}
	return n.below || n.self != ""
}

// rank orders protection reasons sharing one node; higher wins.
func rank(r Reason) int {
	switch r {
	case Volume:
		return 5
	case Home:
		return 4
	case System:
		return 3
	case UserData:
		return 2
	case Custom:
		return 1
	}
	return 0
}

func raise(dst *Reason, r Reason) {
	if rank(r) > rank(*dst) {
		*dst = r
	}
}

// builder compiles the anchors of one host into a trie.
type builder struct {
	fold   bool
	root   *node
	names  map[string]Reason
	goos   string
	home   string // cleaned absolute Home, or ""
	tmp    string // cleaned absolute TempDir, or ""
	uid    string
	mounts []string // cleaned absolute mount points plus the volume root of home
	getenv func(string) string
}

// add anchors reason with kind k at every absolute path in paths; "" and relative paths are skipped,
// as is a content anchor at a volume root, which would recommend the whole volume.
func (b *builder) add(k kind, reason Reason, paths ...string) {
	for _, p := range paths {
		if p = abs(p); p == "" || (k == content && filepath.Dir(p) == p) {
			continue
		}
		if b.fold {
			p = strings.ToLower(p)
		}
		b.anchor(k, reason, p)
		if b.goos == "darwin" {
			if a := privateAlias(p); a != "" {
				b.anchor(k, reason, a)
			}
		}
	}
}

// anchor records reason with kind k at the node of the cleaned, folded absolute path p.
func (b *builder) anchor(k kind, reason Reason, p string) {
	n := b.insert(p)
	switch k {
	case point:
		raise(&n.self, reason)
	case subtree:
		raise(&n.self, reason)
		raise(&n.subtree, reason)
	case content:
		if n.content == "" {
			n.content = reason
		}
	}
}

func (b *builder) insert(p string) *node {
	sep := string(filepath.Separator)
	vol := filepath.VolumeName(p)
	n := b.root.child(vol)
	rest := strings.TrimPrefix(p[len(vol):], sep)
	for rest != "" {
		var seg string
		seg, rest, _ = strings.Cut(rest, sep)
		n = n.child(seg)
	}
	return n
}

// name registers base names; empty, dotted and separator-bearing names are skipped and the first registration wins.
func (b *builder) name(reason Reason, names ...string) {
	for _, s := range names {
		if s == "" || s == "." || s == ".." || strings.ContainsAny(s, `/\`) {
			continue
		}
		if b.fold {
			s = strings.ToLower(s)
		}
		if _, ok := b.names[s]; !ok {
			b.names[s] = reason
		}
	}
}

// h joins parts beneath home, or returns "" when home is unset.
func (b *builder) h(parts ...string) string { return sub(b.home, parts...) }

// env returns the cleaned absolute value of the environment variable name, or def.
func (b *builder) env(name, def string) string {
	if v := abs(b.getenv(name)); v != "" {
		return v
	}
	return def
}

// expand resolves a user path: ~ alone or followed by a separator is relative to home; anything else must be absolute.
func (b *builder) expand(p string) string {
	if p == "~" || (len(p) > 1 && p[0] == '~' && os.IsPathSeparator(p[1])) {
		return sub(b.home, p[1:])
	}
	return abs(p)
}

// abs returns p cleaned when it is absolute, else "".
func abs(p string) string {
	if p == "" {
		return ""
	}
	if p = filepath.Clean(p); !filepath.IsAbs(p) {
		return ""
	}
	return p
}

// sub joins parts beneath base, or returns "" when base is "".
func sub(base string, parts ...string) string {
	if base == "" {
		return ""
	}
	return filepath.Join(append([]string{base}, parts...)...)
}
