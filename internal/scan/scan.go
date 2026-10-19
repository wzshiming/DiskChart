// Package scan walks a directory tree concurrently and aggregates sizes per directory.
package scan

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"
)

// ID identifies a node within a Tree.
type ID int32

// None is the ID of an absent node, such as the root's parent.
const None ID = -1

// Flags mark nodes that were counted or traversed specially.
type Flags uint8

const (
	FlagHardLinkDup Flags = 1 << iota // regular file whose (dev,ino) was already counted; Size and Apparent 0
	FlagSymlink                       // symbolic link, not followed; Size is the link's own allocation, Apparent its Lstat size
	FlagMountPoint                    // directory listed in ExcludeMountpoints, not entered; Size and Apparent 0
	FlagReparse                       // (windows) directory with a reparse point, not entered; Size and Apparent 0
	FlagReadError                     // ReadDir or Lstat failed for this entry; see Tree.Errors
)

// Options control how Scan traverses the file system.
type Options struct {
	SameDevice         bool     // when true, directories in ExcludeMountpoints are not entered (the root always is)
	ExcludeMountpoints []string // absolute, filepath.Clean'ed paths
}

// Progress is a snapshot of the scan counters.
type Progress struct {
	Dirs, Files, Bytes, Errors int64
	CurrentPath                string
	Elapsed                    time.Duration
}

// Error records a path that could not be read.
type Error struct{ Path, Msg string }

// Node is a read-only view of one scanned entry.
type Node struct {
	ID        ID
	Parent    ID     // None for the root
	Name      string // base name; the root holds its cleaned absolute path
	IsDir     bool
	Size      int64 // bytes allocated on disk; for directories the sum of the children's Size, excluding the directory's own blocks
	Apparent  int64 // length reported by Lstat; for directories the sum of the children's Apparent
	FileCount int64 // descendant non-directory nodes (recursive)
	DirCount  int64 // descendant directories (recursive, excluding itself)
	ModTime   int64 // unix seconds (Lstat)
	Flags     Flags
	Children  []ID // nil for non-directories; sorted by Size desc, then Name asc; read-only
}

var (
	ErrNotDir    = errors.New("scan: root is not a directory") // Scan root is not a directory
	ErrInvalidID = errors.New("scan: invalid node id")         // ID is None, out of range, or removed
	ErrRoot      = errors.New("scan: cannot remove root")      // Remove was called on the root
)

const progressInterval = 100 * time.Millisecond

type job struct {
	id   ID
	path string
}

type entry struct {
	name     string
	path     string // set only for directories to enter
	size     int64
	apparent int64
	modTime  int64
	key      fileKey
	hasKey   bool
	isDir    bool
	flags    Flags
}

type scanner struct {
	ctx     context.Context
	exclude map[string]struct{}

	mu      sync.Mutex
	cond    *sync.Cond
	t       *Tree
	stack   []job
	active  int
	stopped bool
	err     error
	seen    map[fileKey]struct{}
	dirs    int64
	files   int64
	bytes   int64
	current string
}

// Scan walks root and returns its size tree. On cancellation it returns the
// consistent partial tree together with ctx.Err().
func Scan(ctx context.Context, root string, opts Options, onProgress func(Progress)) (*Tree, error) {
	start := time.Now()
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	abs = filepath.Clean(abs)
	fi, err := os.Stat(abs)
	if err != nil {
		return nil, err
	}
	if !fi.IsDir() {
		return nil, ErrNotDir
	}

	t := &Tree{root: 0, nodes: make([]node, 1, 1024)}
	t.nodes[0] = node{parent: None, name: abs, isDir: true, modTime: fi.ModTime().Unix()}
	s := &scanner{ctx: ctx, t: t, stack: []job{{0, abs}}, dirs: 1, current: abs}
	s.cond = sync.NewCond(&s.mu)
	if opts.SameDevice && len(opts.ExcludeMountpoints) > 0 {
		s.exclude = make(map[string]struct{}, len(opts.ExcludeMountpoints))
		for _, m := range opts.ExcludeMountpoints {
			s.exclude[filepath.Clean(m)] = struct{}{}
		}
	}

	var (
		last    time.Time
		called  bool
		stop    = make(chan struct{})
		repDone = make(chan struct{})
	)
	if onProgress != nil {
		go func() {
			defer close(repDone)
			next := start.Add(progressInterval)
			timer := time.NewTimer(progressInterval)
			defer timer.Stop()
			for {
				select {
				case <-stop:
					return
				case <-timer.C:
				}
				if now := time.Now(); now.Before(next) {
					timer.Reset(next.Sub(now))
					continue
				}
				p := s.snapshot(start)
				last, called = time.Now(), true
				onProgress(p)
				next = last.Add(progressInterval)
				timer.Reset(time.Until(next))
			}
		}()
	}

	var wg sync.WaitGroup
	for range max(2*runtime.NumCPU(), 4) {
		wg.Go(s.work)
	}
	wg.Wait()
	t.finish()

	if onProgress != nil {
		close(stop)
		<-repDone
		if called {
			if d := progressInterval - time.Since(last); d > 0 {
				time.Sleep(d)
			}
		}
		r := &t.nodes[t.root]
		onProgress(Progress{
			Dirs:        1 + r.dirCount,
			Files:       r.fileCount,
			Bytes:       r.size,
			Errors:      int64(len(t.errs)),
			CurrentPath: s.current,
			Elapsed:     time.Since(start),
		})
	}
	return t, s.err
}

func (s *scanner) snapshot(start time.Time) Progress {
	s.mu.Lock()
	defer s.mu.Unlock()
	return Progress{
		Dirs:        s.dirs,
		Files:       s.files,
		Bytes:       s.bytes,
		Errors:      int64(len(s.t.errs)),
		CurrentPath: s.current,
		Elapsed:     time.Since(start),
	}
}

func (s *scanner) work() {
	for {
		s.mu.Lock()
		for len(s.stack) == 0 && s.active > 0 && !s.stopped {
			s.cond.Wait()
		}
		if !s.stopped && s.ctx.Err() != nil {
			s.stopped, s.err = true, s.ctx.Err()
		}
		if s.stopped || len(s.stack) == 0 {
			s.stopped = true
			s.cond.Broadcast()
			s.mu.Unlock()
			return
		}
		j := s.stack[len(s.stack)-1]
		s.stack = s.stack[:len(s.stack)-1]
		s.active++
		s.mu.Unlock()

		s.readDir(j)
	}
}

func readEntries(dir string) ([]fs.DirEntry, error) {
	f, err := os.Open(dir)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return f.ReadDir(-1)
}

func (s *scanner) readDir(j job) {
	des, rerr := readEntries(j.path)
	var errs []Error
	if rerr != nil {
		errs = append(errs, Error{Path: j.path, Msg: rerr.Error()})
	}
	items := make([]entry, 0, len(des))
	for _, d := range des {
		name := d.Name()
		info, err := d.Info()
		if err != nil {
			items = append(items, entry{name: name, isDir: d.IsDir(), flags: FlagReadError})
			errs = append(errs, Error{Path: filepath.Join(j.path, name), Msg: err.Error()})
			continue
		}
		e := entry{name: name, modTime: info.ModTime().Unix()}
		mode := info.Mode()
		switch {
		case mode&fs.ModeSymlink != 0:
			e.flags = FlagSymlink
			e.size = allocatedSize(filepath.Join(j.path, name), info)
			e.apparent = info.Size()
		case isReparseDir(info):
			e.isDir = true
			e.flags = FlagReparse
		case mode.IsDir():
			e.isDir = true
			p := filepath.Join(j.path, name)
			if _, ok := s.exclude[p]; ok {
				e.flags = FlagMountPoint
			} else {
				e.path = p
			}
		default:
			e.size = allocatedSize(filepath.Join(j.path, name), info)
			e.apparent = info.Size()
			e.key, e.hasKey = hardLinkKey(info)
		}
		items = append(items, e)
	}

	s.mu.Lock()
	t := s.t
	t.errs = append(t.errs, errs...)
	if rerr != nil {
		t.nodes[j.id].flags |= FlagReadError
	}
	var children []ID
	if len(items) > 0 {
		children = make([]ID, 0, len(items))
	}
	for _, e := range items {
		id := ID(len(t.nodes))
		if e.hasKey {
			if _, dup := s.seen[e.key]; dup {
				e.flags |= FlagHardLinkDup
				e.size, e.apparent = 0, 0
			} else {
				if s.seen == nil {
					s.seen = make(map[fileKey]struct{})
				}
				s.seen[e.key] = struct{}{}
			}
		}
		t.nodes = append(t.nodes, node{
			name:     e.name,
			parent:   j.id,
			isDir:    e.isDir,
			size:     e.size,
			apparent: e.apparent,
			modTime:  e.modTime,
			flags:    e.flags,
		})
		children = append(children, id)
		if e.isDir {
			s.dirs++
		} else {
			s.files++
			s.bytes += e.size
		}
		if e.path != "" {
			s.stack = append(s.stack, job{id, e.path})
		}
	}
	t.nodes[j.id].children = children
	s.current = j.path
	s.active--
	s.cond.Broadcast()
	s.mu.Unlock()
}
