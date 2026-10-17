// Package app is the Wails-bound service that runs scans and answers frontend queries.
package app

import (
	"context"
	"errors"
	"os"
	"sync"
	"time"

	"github.com/wzshiming/DiskChart/internal/fsops"
	"github.com/wzshiming/DiskChart/internal/scan"
	"github.com/wzshiming/DiskChart/internal/stats"
	"github.com/wzshiming/DiskChart/internal/volumes"
)

// Event names emitted to the frontend.
const (
	EventScanProgress = "scan:progress"
	EventScanFinished = "scan:finished"
	EventTreeChanged  = "tree:changed"
)

// Scan lifecycle statuses reported in ScanState.Status.
const (
	StatusIdle      = "idle"
	StatusScanning  = "scanning"
	StatusDone      = "done"
	StatusCancelled = "cancelled"
	StatusFailed    = "failed"
)

// Errors returned by App methods.
var (
	ErrScanning   = errors.New("app: scan already running")
	ErrNoScan     = errors.New("app: no scan running")
	ErrNoTree     = errors.New("app: no scan result")
	ErrInvalidID  = errors.New("app: invalid node id")
	ErrRootDelete = errors.New("app: refusing to delete scan root")
)

var errEmptyRoot = errors.New("app: empty scan root")

// Deps are App's collaborators; New fills zero fields with the real implementations.
type Deps struct {
	Scan        func(ctx context.Context, root string, opts scan.Options, onProgress func(scan.Progress)) (*scan.Tree, error)
	ListVolumes func(ctx context.Context) ([]volumes.Volume, error)
	Mountpoints func(ctx context.Context) ([]string, error)
	Reveal      func(ctx context.Context, path string) error
	Delete      func(g fsops.Guard, path string) error
	Home        string
	Version     string
}

// App manages one scan at a time and serves queries over its result.
type App struct {
	rt   Runtime
	deps Deps

	mu          sync.Mutex
	state       ScanState
	tree        *scan.Tree
	cancel      context.CancelFunc
	mountpoints []string
}

// New returns an idle App using rt and deps.
func New(rt Runtime, deps Deps) *App {
	if deps.Scan == nil {
		deps.Scan = scan.Scan
	}
	if deps.ListVolumes == nil {
		deps.ListVolumes = volumes.List
	}
	if deps.Mountpoints == nil {
		deps.Mountpoints = volumes.Mountpoints
	}
	if deps.Reveal == nil {
		deps.Reveal = fsops.Reveal
	}
	if deps.Delete == nil {
		deps.Delete = func(g fsops.Guard, p string) error { return g.Delete(p) }
	}
	if deps.Home == "" {
		deps.Home, _ = os.UserHomeDir()
	}
	if deps.Version == "" {
		deps.Version = "dev"
	}
	return &App{rt: rt, deps: deps, state: ScanState{Status: StatusIdle, RootID: -1}}
}

// Info returns the version and platform.
func (a *App) Info() Info {
	return Info{Version: a.deps.Version, Platform: a.rt.Platform()}
}

// EventPayloads is never called by the frontend; it exists only for type generation.
func (a *App) EventPayloads() EventPayloads { return EventPayloads{} }

// ListVolumes returns the scannable volumes.
func (a *App) ListVolumes() ([]Volume, error) {
	vs, err := a.deps.ListVolumes(context.Background())
	if err != nil {
		return nil, err
	}
	out := make([]Volume, len(vs))
	for i, v := range vs {
		out[i] = toVolume(v)
	}
	return out, nil
}

// PickDirectory shows a directory picker; "" means the user cancelled.
func (a *App) PickDirectory(title string) (string, error) {
	return a.rt.OpenDirectoryDialog(title)
}

// StartScan begins scanning root in the background.
func (a *App) StartScan(root string, opts ScanOptions) error {
	if root == "" {
		return errEmptyRoot
	}
	a.mu.Lock()
	if a.state.Status == StatusScanning {
		a.mu.Unlock()
		return ErrScanning
	}
	ctx, cancel := context.WithCancel(context.Background())
	a.tree = nil
	a.cancel = cancel
	a.mountpoints = nil
	a.state = ScanState{Status: StatusScanning, Root: root, RootID: -1}
	a.mu.Unlock()

	mps, err := a.deps.Mountpoints(ctx)
	if err != nil {
		mps = []string{}
	}
	a.mu.Lock()
	a.mountpoints = mps
	a.mu.Unlock()

	go a.run(ctx, cancel, root, scan.Options{SameDevice: opts.SameDevice, ExcludeMountpoints: mps})
	return nil
}

func (a *App) run(ctx context.Context, cancel context.CancelFunc, root string, opts scan.Options) {
	defer cancel()
	start := time.Now()
	tree, err := a.deps.Scan(ctx, root, opts, func(sp scan.Progress) {
		p := toProgress(sp)
		a.mu.Lock()
		a.state.Progress = p
		a.mu.Unlock()
		a.rt.EventsEmit(EventScanProgress, p)
	})
	dur := time.Since(start)

	a.mu.Lock()
	s := &a.state
	switch {
	case err == nil && tree != nil:
		s.Status = StatusDone
	case tree != nil && (errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)):
		s.Status = StatusCancelled
	default:
		s.Status = StatusFailed
		if err != nil {
			s.Error = err.Error()
		} else {
			s.Error = "app: scan returned no result"
		}
	}
	if s.Status == StatusFailed {
		a.tree = nil
		s.RootID = -1
	} else {
		a.tree = tree
		r, _ := tree.Node(tree.Root())
		s.RootID = int32(tree.Root())
		s.Summary = Summary{
			Size:       r.Size,
			Files:      r.FileCount,
			Dirs:       r.DirCount + 1,
			Errors:     int64(len(tree.Errors())),
			DurationMs: dur.Milliseconds(),
		}
	}
	a.cancel = nil
	ev := FinishedEvent{Status: s.Status, Error: s.Error, Root: root, RootID: s.RootID, Summary: s.Summary}
	a.mu.Unlock()
	a.rt.EventsEmit(EventScanFinished, ev)
}

// CancelScan stops the running scan; its state updates when the scan returns.
func (a *App) CancelScan() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.state.Status != StatusScanning || a.cancel == nil {
		return ErrNoScan
	}
	a.cancel()
	return nil
}

// State returns a copy of the scan state.
func (a *App) State() ScanState {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.state
}

// node returns the tree and node for id; the caller must hold a.mu.
func (a *App) node(id int32) (*scan.Tree, scan.Node, error) {
	if a.tree == nil {
		return nil, scan.Node{}, ErrNoTree
	}
	n, ok := a.tree.Node(scan.ID(id))
	if !ok {
		return nil, scan.Node{}, ErrInvalidID
	}
	return a.tree, n, nil
}

// Subtree returns a bounded view of the subtree at id.
func (a *App) Subtree(id int32, depth, maxChildren int, minFraction float64) (View, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	t, _, err := a.node(id)
	if err != nil {
		return View{}, err
	}
	v, ok := stats.Subtree(t, scan.ID(id), depth, maxChildren, minFraction)
	if !ok {
		return View{}, ErrInvalidID
	}
	return toView(v), nil
}

// maxChildrenPage caps the limit accepted by Children.
const maxChildrenPage = 1000

// Children returns the live children of directory id in display order, paged by offset and limit.
func (a *App) Children(id int32, offset, limit int) ([]Entry, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	t, _, err := a.node(id)
	if err != nil {
		return nil, err
	}
	if limit <= 0 {
		return []Entry{}, nil
	}
	es, ok := stats.Children(t, scan.ID(id), max(offset, 0), min(limit, maxChildrenPage))
	if !ok {
		return nil, ErrInvalidID
	}
	return toEntries(es), nil
}

// Crumbs returns the path from the root to id.
func (a *App) Crumbs(id int32) ([]Crumb, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	t, _, err := a.node(id)
	if err != nil {
		return nil, err
	}
	cs := stats.Crumbs(t, scan.ID(id))
	if cs == nil {
		return nil, ErrInvalidID
	}
	out := make([]Crumb, len(cs))
	for i, c := range cs {
		out[i] = Crumb{ID: int32(c.ID), Name: c.Name}
	}
	return out, nil
}

// TopFiles returns the n largest files.
func (a *App) TopFiles(n int) ([]Entry, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.tree == nil {
		return nil, ErrNoTree
	}
	return toEntries(stats.TopFiles(a.tree, n)), nil
}

// TopDirs returns the n largest directories.
func (a *App) TopDirs(n int) ([]Entry, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.tree == nil {
		return nil, ErrNoTree
	}
	return toEntries(stats.TopDirs(a.tree, n)), nil
}

// Extensions returns the size breakdown by extension, keeping the topK largest.
func (a *App) Extensions(topK int) (ExtReport, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.tree == nil {
		return ExtReport{}, ErrNoTree
	}
	r := stats.ByExtension(a.tree, topK)
	out := ExtReport{Items: make([]ExtStat, len(r.Items)), Other: toExtStat(r.Other), Total: r.Total}
	for i, e := range r.Items {
		out.Items[i] = toExtStat(e)
	}
	return out, nil
}

// ScanErrors returns up to limit read errors; limit <= 0 returns all.
func (a *App) ScanErrors(limit int) ([]ScanError, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.tree == nil {
		return nil, ErrNoTree
	}
	errs := a.tree.Errors()
	if limit > 0 && len(errs) > limit {
		errs = errs[:limit]
	}
	out := make([]ScanError, len(errs))
	for i, e := range errs {
		out[i] = ScanError{Path: e.Path, Msg: e.Msg}
	}
	return out, nil
}

// NodePath returns the absolute path of id.
func (a *App) NodePath(id int32) (string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	t, _, err := a.node(id)
	if err != nil {
		return "", err
	}
	return t.Path(scan.ID(id)), nil
}

// Reveal shows id in the system file manager.
func (a *App) Reveal(id int32) error {
	p, err := a.NodePath(id)
	if err != nil {
		return err
	}
	return a.deps.Reveal(context.Background(), p)
}

// CopyPath copies the absolute path of id to the clipboard.
func (a *App) CopyPath(id int32) error {
	p, err := a.NodePath(id)
	if err != nil {
		return err
	}
	return a.rt.ClipboardSetText(p)
}

// Delete permanently removes id from disk and from the tree.
func (a *App) Delete(id int32) (DeleteResult, error) {
	a.mu.Lock()
	t, n, err := a.node(id)
	if err != nil {
		a.mu.Unlock()
		return DeleteResult{}, err
	}
	if scan.ID(id) == t.Root() {
		a.mu.Unlock()
		return DeleteResult{}, ErrRootDelete
	}
	p := t.Path(scan.ID(id))
	g := fsops.Guard{Mountpoints: a.mountpoints, Home: a.deps.Home}
	if err := a.deps.Delete(g, p); err != nil {
		if !errors.Is(err, fsops.ErrRefused) {
			a.state.TreeStale = true
		}
		a.mu.Unlock()
		return DeleteResult{}, err
	}
	parent, err := t.Remove(scan.ID(id))
	if err != nil {
		a.state.TreeStale = true
		a.mu.Unlock()
		return DeleteResult{}, err
	}
	a.mu.Unlock()

	res := DeleteResult{ParentID: int32(parent), RemovedSize: n.Size, RemovedFiles: 1}
	if n.IsDir {
		res.RemovedFiles, res.RemovedDirs = n.FileCount, n.DirCount+1
	}
	a.rt.EventsEmit(EventTreeChanged, TreeChangedEvent{RemovedID: id, ParentID: int32(parent)})
	return res, nil
}
