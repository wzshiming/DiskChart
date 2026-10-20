package app

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/wzshiming/DiskChart/internal/fsops"
	"github.com/wzshiming/DiskChart/internal/scan"
	"github.com/wzshiming/DiskChart/internal/volumes"
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

// fixture, sizes in units: a/f1=100 a/f2=200 b/f3=1000 r.txt=7 (Size 1307, FileCount 4, DirCount 2)
func fixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "a", "f1"), 100)
	writeFile(t, filepath.Join(root, "a", "f2"), 200)
	writeFile(t, filepath.Join(root, "b", "f3"), 1000)
	writeFile(t, filepath.Join(root, "r.txt"), 7)
	return root
}

func fixtureTree(t *testing.T, root string) *scan.Tree {
	t.Helper()
	tree, err := scan.Scan(context.Background(), root, scan.Options{}, nil)
	if err != nil || tree == nil {
		t.Fatalf("scan.Scan(%q) = %v, %v", root, tree, err)
	}
	return tree
}

func noMounts(context.Context) ([]string, error) { return nil, nil }

func waitState(t *testing.T, a *App) ScanState {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		st := a.State()
		if st.Status != StatusScanning {
			return st
		}
		if time.Now().After(deadline) {
			t.Fatalf("scan still running after 5s: %+v", st)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func waitFinished(t *testing.T, a *App, rt *fakeRuntime) ScanState {
	t.Helper()
	select {
	case <-rt.finished:
	case <-time.After(5 * time.Second):
		t.Fatal("no scan:finished event within 5s")
	}
	return waitState(t, a)
}

// scanDone runs the default (real) scan over a fresh fixture and waits for it to finish.
func scanDone(t *testing.T, deps Deps) (*App, *fakeRuntime, string, ScanState) {
	t.Helper()
	root := fixture(t)
	rt := newFakeRuntime()
	if deps.Mountpoints == nil {
		deps.Mountpoints = noMounts
	}
	a := New(rt, deps)
	if err := a.StartScan(root, ScanOptions{}); err != nil {
		t.Fatalf("StartScan: %v", err)
	}
	st := waitFinished(t, a, rt)
	if st.Status != StatusDone {
		t.Fatalf("Status = %q (err %q), want %q", st.Status, st.Error, StatusDone)
	}
	return a, rt, root, st
}

func child(t *testing.T, a *App, parent int32, name string) View {
	t.Helper()
	v, err := a.Subtree(parent, 1, 10, 0)
	if err != nil {
		t.Fatalf("Subtree(%d): %v", parent, err)
	}
	for _, c := range v.Children {
		if c.Name == name && !c.Aggregate {
			return c
		}
	}
	t.Fatalf("child %q not found under %d: %+v", name, parent, v.Children)
	return View{}
}

// blockingScan blocks until ctx is cancelled, then returns tree and ctx.Err().
func blockingScan(tree *scan.Tree) func(context.Context, string, scan.Options, func(scan.Progress)) (*scan.Tree, error) {
	return func(ctx context.Context, _ string, _ scan.Options, _ func(scan.Progress)) (*scan.Tree, error) {
		<-ctx.Done()
		return tree, ctx.Err()
	}
}

// startBlocked starts a scan that runs until cancelled; cleanup cancels it.
func startBlocked(t *testing.T, tree *scan.Tree) (*App, *fakeRuntime, string) {
	t.Helper()
	rt := newFakeRuntime()
	a := New(rt, Deps{Scan: blockingScan(tree), Mountpoints: noMounts})
	root := t.TempDir()
	if err := a.StartScan(root, ScanOptions{}); err != nil {
		t.Fatalf("StartScan: %v", err)
	}
	t.Cleanup(func() {
		_ = a.CancelScan()
		waitState(t, a)
	})
	return a, rt, root
}

func TestNewInitialStateIsIdle(t *testing.T) {
	a := New(newFakeRuntime(), Deps{})
	st := a.State()
	if st.Status != StatusIdle || st.RootID != -1 || st.TreeStale {
		t.Fatalf("initial State = %+v, want Status idle, RootID -1, TreeStale false", st)
	}
}

func TestInfoReportsVersionAndPlatform(t *testing.T) {
	rt := newFakeRuntime()
	rt.platform = "darwin"
	got := New(rt, Deps{Version: "1.2.3"}).Info()
	if want := (Info{Version: "1.2.3", Platform: "darwin"}); got != want {
		t.Fatalf("Info() = %+v, want %+v", got, want)
	}
}

func TestListVolumesMapsDeps(t *testing.T) {
	a := New(newFakeRuntime(), Deps{ListVolumes: func(context.Context) ([]volumes.Volume, error) {
		return []volumes.Volume{{Mountpoint: "/m", Device: "/dev/x", FSType: "apfs", Label: "Mac", Total: 300, Used: 100, Free: 200}}, nil
	}})
	got, err := a.ListVolumes()
	if err != nil {
		t.Fatalf("ListVolumes: %v", err)
	}
	want := []Volume{{Mountpoint: "/m", Label: "Mac", FSType: "apfs", Total: 300, Used: 100, Free: 200}}
	if !slices.Equal(got, want) {
		t.Fatalf("ListVolumes() = %+v, want %+v", got, want)
	}
}

func TestListVolumesPassesError(t *testing.T) {
	boom := errors.New("boom")
	a := New(newFakeRuntime(), Deps{ListVolumes: func(context.Context) ([]volumes.Volume, error) { return nil, boom }})
	if _, err := a.ListVolumes(); !errors.Is(err, boom) {
		t.Fatalf("ListVolumes() error = %v, want %v", err, boom)
	}
}

func TestPickDirectoryPassesThroughRuntime(t *testing.T) {
	rt := newFakeRuntime()
	rt.dir = "/picked"
	a := New(rt, Deps{})
	got, err := a.PickDirectory("Choose")
	if got != "/picked" || err != nil || rt.dirTitle != "Choose" {
		t.Fatalf("PickDirectory = %q, %v (title %q), want /picked, nil (title Choose)", got, err, rt.dirTitle)
	}
	boom := errors.New("boom")
	rt.dirErr = boom
	if _, err := a.PickDirectory("x"); !errors.Is(err, boom) {
		t.Fatalf("PickDirectory error = %v, want %v", err, boom)
	}
}

func TestStartScanEmptyRootFails(t *testing.T) {
	a := New(newFakeRuntime(), Deps{Mountpoints: noMounts})
	if err := a.StartScan("", ScanOptions{}); err == nil {
		t.Fatal(`StartScan("") = nil, want error`)
	}
}

func TestStartScanReturnsImmediatelyScanning(t *testing.T) {
	a, _, root := startBlocked(t, nil)
	st := a.State()
	if st.Status != StatusScanning || st.Root != root {
		t.Fatalf("State during scan = %+v, want Status scanning, Root %q", st, root)
	}
}

func TestStartScanWhileScanningIsErrScanning(t *testing.T) {
	a, _, _ := startBlocked(t, nil)
	if err := a.StartScan(t.TempDir(), ScanOptions{}); !errors.Is(err, ErrScanning) {
		t.Fatalf("second StartScan error = %v, want ErrScanning", err)
	}
}

func TestQueriesWhileScanningAreErrNoTree(t *testing.T) {
	a, _, _ := startBlocked(t, nil)
	if _, err := a.Subtree(0, 1, 10, 0); !errors.Is(err, ErrNoTree) {
		t.Errorf("Subtree error = %v, want ErrNoTree", err)
	}
	if _, err := a.TopFiles(1); !errors.Is(err, ErrNoTree) {
		t.Errorf("TopFiles error = %v, want ErrNoTree", err)
	}
	if _, err := a.Extensions(1); !errors.Is(err, ErrNoTree) {
		t.Errorf("Extensions error = %v, want ErrNoTree", err)
	}
}

func TestStartScanSameDeviceExcludesMountpoints(t *testing.T) {
	mounts := []string{"/mnt/one", "/mnt/two"}
	var mu sync.Mutex
	var got scan.Options
	called := make(chan struct{})
	a := New(newFakeRuntime(), Deps{
		Mountpoints: func(context.Context) ([]string, error) { return mounts, nil },
		Scan: func(_ context.Context, _ string, opts scan.Options, _ func(scan.Progress)) (*scan.Tree, error) {
			mu.Lock()
			got = opts
			mu.Unlock()
			close(called)
			return nil, errors.New("stop")
		},
	})
	if err := a.StartScan(t.TempDir(), ScanOptions{SameDevice: true}); err != nil {
		t.Fatalf("StartScan: %v", err)
	}
	select {
	case <-called:
	case <-time.After(5 * time.Second):
		t.Fatal("Deps.Scan not called")
	}
	waitState(t, a)
	mu.Lock()
	defer mu.Unlock()
	if !got.SameDevice || !slices.Equal(got.ExcludeMountpoints, mounts) {
		t.Fatalf("scan.Options = %+v, want SameDevice true, ExcludeMountpoints %v", got, mounts)
	}
}

func TestScanDoneStateAndSummary(t *testing.T) {
	_, _, _, st := scanDone(t, Deps{})
	if st.RootID < 0 || st.Error != "" {
		t.Fatalf("State = %+v, want RootID >= 0 and no Error", st)
	}
	s := st.Summary
	if s.Size != 1307*unit || s.Files != 4 || s.Dirs != 3 || s.Errors != 0 || s.DurationMs < 0 {
		t.Fatalf("Summary = %+v, want Size %d, Files 4, Dirs 3 (incl. root), Errors 0, DurationMs >= 0", s, 1307*unit)
	}
}

func TestSummaryDirsIncludesRootLikeProgress(t *testing.T) {
	a, _, _, st := scanDone(t, Deps{})
	v, err := a.Subtree(st.RootID, 1, 10, 0)
	if err != nil {
		t.Fatalf("Subtree: %v", err)
	}
	dirs := int64(0)
	for _, c := range v.Children {
		if c.IsDir {
			dirs++
		}
	}
	if st.Summary.Dirs != dirs+1 || st.Summary.Dirs != st.Progress.Dirs {
		t.Fatalf("Summary.Dirs = %d, want root DirCount+1 = %d and final Progress.Dirs = %d", st.Summary.Dirs, dirs+1, st.Progress.Dirs)
	}
}

func TestScanDoneEmitsProgressEvents(t *testing.T) {
	_, rt, _, _ := scanDone(t, Deps{})
	evs := rt.eventsNamed(EventScanProgress)
	if len(evs) == 0 {
		t.Fatal("no scan:progress events")
	}
	for _, e := range evs {
		if _, ok := e.Data.(Progress); !ok {
			t.Fatalf("scan:progress data type %T, want Progress", e.Data)
		}
	}
}

func TestScanDoneEmitsOneFinishedEvent(t *testing.T) {
	_, rt, _, st := scanDone(t, Deps{})
	evs := rt.eventsNamed(EventScanFinished)
	if len(evs) != 1 {
		t.Fatalf("scan:finished emitted %d times, want 1", len(evs))
	}
	fe, ok := evs[0].Data.(FinishedEvent)
	if !ok {
		t.Fatalf("scan:finished data type %T, want FinishedEvent", evs[0].Data)
	}
	if fe.Status != StatusDone || fe.RootID != st.RootID || fe.Summary != st.Summary {
		t.Fatalf("FinishedEvent = %+v, want Status done, RootID %d, Summary %+v", fe, st.RootID, st.Summary)
	}
}

func TestCancelScanWhenIdleIsErrNoScan(t *testing.T) {
	a := New(newFakeRuntime(), Deps{})
	if err := a.CancelScan(); !errors.Is(err, ErrNoScan) {
		t.Fatalf("CancelScan() = %v, want ErrNoScan", err)
	}
}

func TestCancelScanEndsCancelledWithPartialTree(t *testing.T) {
	tree := fixtureTree(t, fixture(t))
	a, rt, _ := startBlocked(t, tree)
	if err := a.CancelScan(); err != nil {
		t.Fatalf("CancelScan() = %v, want nil", err)
	}
	st := waitFinished(t, a, rt)
	if st.Status != StatusCancelled || st.RootID < 0 {
		t.Fatalf("State after cancel = %+v, want Status cancelled, RootID >= 0", st)
	}
}

func TestScanFailureEndsFailed(t *testing.T) {
	rt := newFakeRuntime()
	a := New(rt, Deps{Mountpoints: noMounts, Scan: func(context.Context, string, scan.Options, func(scan.Progress)) (*scan.Tree, error) {
		return nil, errors.New("boom")
	}})
	if err := a.StartScan(t.TempDir(), ScanOptions{}); err != nil {
		t.Fatalf("StartScan: %v", err)
	}
	st := waitFinished(t, a, rt)
	if st.Status != StatusFailed || st.Error != "boom" || st.RootID != -1 {
		t.Fatalf("State = %+v, want Status failed, Error boom, RootID -1", st)
	}
	evs := rt.eventsNamed(EventScanFinished)
	if len(evs) != 1 {
		t.Fatalf("scan:finished emitted %d times, want 1", len(evs))
	}
	if fe, ok := evs[0].Data.(FinishedEvent); !ok || fe.Status != StatusFailed {
		t.Fatalf("scan:finished data = %#v, want FinishedEvent with Status failed", evs[0].Data)
	}
}

func TestScanFailureDiscardsReturnedTree(t *testing.T) {
	tree := fixtureTree(t, fixture(t))
	rt := newFakeRuntime()
	a := New(rt, Deps{Mountpoints: noMounts, Scan: func(context.Context, string, scan.Options, func(scan.Progress)) (*scan.Tree, error) {
		return tree, errors.New("boom")
	}})
	if err := a.StartScan(t.TempDir(), ScanOptions{}); err != nil {
		t.Fatalf("StartScan: %v", err)
	}
	st := waitFinished(t, a, rt)
	if st.Status != StatusFailed || st.RootID != -1 {
		t.Fatalf("State = %+v, want Status failed, RootID -1", st)
	}
	if _, err := a.Subtree(0, 1, 10, 0); !errors.Is(err, ErrNoTree) {
		t.Fatalf("Subtree(0) after failed scan error = %v, want ErrNoTree", err)
	}
	evs := rt.eventsNamed(EventScanFinished)
	if len(evs) != 1 {
		t.Fatalf("scan:finished emitted %d times, want 1", len(evs))
	}
	if fe, ok := evs[0].Data.(FinishedEvent); !ok || fe.Status != StatusFailed || fe.RootID != -1 {
		t.Fatalf("scan:finished data = %#v, want Status failed, RootID -1", evs[0].Data)
	}
}

func TestCancelledFinishedEventPayload(t *testing.T) {
	tree := fixtureTree(t, fixture(t))
	r, _ := tree.Node(tree.Root())
	a, rt, _ := startBlocked(t, tree)
	if err := a.CancelScan(); err != nil {
		t.Fatalf("CancelScan() = %v, want nil", err)
	}
	waitFinished(t, a, rt)
	evs := rt.eventsNamed(EventScanFinished)
	if len(evs) != 1 {
		t.Fatalf("scan:finished emitted %d times, want 1", len(evs))
	}
	fe, ok := evs[0].Data.(FinishedEvent)
	if !ok || fe.Status != StatusCancelled || fe.RootID < 0 || fe.Summary.Size != r.Size {
		t.Fatalf("scan:finished data = %#v, want Status cancelled, RootID >= 0, Summary.Size %d", evs[0].Data, r.Size)
	}
}

func TestCancelScanAfterDoneIsErrNoScan(t *testing.T) {
	a, _, _, _ := scanDone(t, Deps{})
	if err := a.CancelScan(); !errors.Is(err, ErrNoScan) {
		t.Fatalf("CancelScan() after done = %v, want ErrNoScan", err)
	}
}

func TestStartScanAgainResetsState(t *testing.T) {
	tree := fixtureTree(t, fixture(t))
	release := make(chan struct{})
	var mu sync.Mutex
	calls := 0
	rt := newFakeRuntime()
	a := New(rt, Deps{
		Mountpoints: noMounts,
		Delete:      func(fsops.Guard, string) error { return errors.New("boom") },
		Scan: func(_ context.Context, _ string, _ scan.Options, onProgress func(scan.Progress)) (*scan.Tree, error) {
			mu.Lock()
			calls++
			n := calls
			mu.Unlock()
			if n == 1 {
				onProgress(scan.Progress{Dirs: 3, Files: 4, Bytes: 1307, CurrentPath: "x"})
				return tree, nil
			}
			<-release
			return tree, nil
		},
	})
	if err := a.StartScan(t.TempDir(), ScanOptions{}); err != nil {
		t.Fatalf("first StartScan: %v", err)
	}
	first := waitFinished(t, a, rt)
	if first.Status != StatusDone {
		t.Fatalf("first scan Status = %q, want done", first.Status)
	}
	f := child(t, a, first.RootID, "r.txt")
	if _, err := a.Delete(f.ID); err == nil || !a.State().TreeStale {
		t.Fatalf("setup: failed Delete should mark TreeStale, err %v", err)
	}

	root2 := t.TempDir()
	if err := a.StartScan(root2, ScanOptions{}); err != nil {
		t.Fatalf("second StartScan: %v", err)
	}
	st := a.State()
	want := ScanState{Status: StatusScanning, Root: root2, RootID: -1}
	if st != want {
		close(release)
		waitState(t, a)
		t.Fatalf("State after second StartScan = %+v, want %+v", st, want)
	}
	_, err := a.Subtree(0, 1, 10, 0)
	close(release)
	waitState(t, a)
	if !errors.Is(err, ErrNoTree) {
		t.Fatalf("Subtree(0) during second scan error = %v, want ErrNoTree (old tree dropped)", err)
	}
}

func TestMountpointsFailureContinues(t *testing.T) {
	tree := fixtureTree(t, fixture(t))
	var mu sync.Mutex
	var got scan.Options
	rt := newFakeRuntime()
	a := New(rt, Deps{
		Mountpoints: func(context.Context) ([]string, error) { return []string{"/mnt/x"}, errors.New("boom") },
		Scan: func(_ context.Context, _ string, opts scan.Options, _ func(scan.Progress)) (*scan.Tree, error) {
			mu.Lock()
			got = opts
			mu.Unlock()
			return tree, nil
		},
	})
	if err := a.StartScan(t.TempDir(), ScanOptions{SameDevice: true}); err != nil {
		t.Fatalf("StartScan with failing Mountpoints = %v, want nil", err)
	}
	if st := waitFinished(t, a, rt); st.Status != StatusDone {
		t.Fatalf("Status = %q (err %q), want done", st.Status, st.Error)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(got.ExcludeMountpoints) != 0 {
		t.Fatalf("ExcludeMountpoints = %v, want empty when Mountpoints fails", got.ExcludeMountpoints)
	}
}

func TestScanErrorsLimitNoErrors(t *testing.T) {
	tree := fixtureTree(t, fixture(t))
	rt := newFakeRuntime()
	a := New(rt, Deps{Mountpoints: noMounts, Scan: func(context.Context, string, scan.Options, func(scan.Progress)) (*scan.Tree, error) {
		return tree, nil
	}})
	if err := a.StartScan(t.TempDir(), ScanOptions{}); err != nil {
		t.Fatalf("StartScan: %v", err)
	}
	waitFinished(t, a, rt)
	for _, n := range []int{0, 5} {
		es, err := a.ScanErrors(n)
		if err != nil || len(es) != 0 {
			t.Fatalf("ScanErrors(%d) = %+v, %v, want empty", n, es, err)
		}
	}
}

func TestSubtreeRootSizeAndOrder(t *testing.T) {
	a, _, _, st := scanDone(t, Deps{})
	v, err := a.Subtree(st.RootID, 1, 10, 0)
	if err != nil {
		t.Fatalf("Subtree: %v", err)
	}
	if v.Size != st.Summary.Size || len(v.Children) == 0 {
		t.Fatalf("Subtree root Size %d with %d children, want Size %d and children", v.Size, len(v.Children), st.Summary.Size)
	}
	if !slices.IsSortedFunc(v.Children, func(x, y View) int { return int(y.Size - x.Size) }) {
		t.Fatalf("Subtree children not sorted by Size desc: %+v", v.Children)
	}
	for _, x := range append([]View{v}, v.Children...) {
		if x.Apparent != x.Size {
			t.Errorf("Subtree(root) view %q Apparent = %d, want Size %d", x.Name, x.Apparent, x.Size)
		}
	}
}

func TestSubtreeChildrenNeverNil(t *testing.T) {
	a, _, _, st := scanDone(t, Deps{})
	f := child(t, a, st.RootID, "r.txt")
	v, err := a.Subtree(f.ID, 1, 10, 0)
	if err != nil {
		t.Fatalf("Subtree(file): %v", err)
	}
	if v.Children == nil || len(v.Children) != 0 {
		t.Fatalf("Subtree(file).Children = %#v, want non-nil empty slice", v.Children)
	}
	d := child(t, a, st.RootID, "a")
	v, err = a.Subtree(d.ID, 0, 10, 0)
	if err != nil {
		t.Fatalf("Subtree(dir, depth 0): %v", err)
	}
	if v.Children == nil || len(v.Children) != 0 {
		t.Fatalf("Subtree(dir, depth 0).Children = %#v, want non-nil empty slice", v.Children)
	}
	v, err = a.Subtree(st.RootID, 1, 10, 0)
	if err != nil {
		t.Fatalf("Subtree(root): %v", err)
	}
	for _, c := range v.Children {
		if c.Children == nil {
			t.Fatalf("unexpanded child %q Children = nil, want non-nil empty slice", c.Name)
		}
	}
}

func TestSubtreeInvalidID(t *testing.T) {
	a, _, _, _ := scanDone(t, Deps{})
	for _, id := range []int32{-1, 1 << 30} {
		if _, err := a.Subtree(id, 1, 10, 0); !errors.Is(err, ErrInvalidID) {
			t.Errorf("Subtree(%d) error = %v, want ErrInvalidID", id, err)
		}
	}
}

func TestSubtreeSkipExposed(t *testing.T) {
	a, _, _, st := scanDone(t, Deps{})
	v, err := a.Subtree(st.RootID, 1, 1, 0)
	if err != nil {
		t.Fatalf("Subtree: %v", err)
	}
	if len(v.Children) != 2 {
		t.Fatalf("Subtree(root, 1, 1, 0).Children = %+v, want b and aggregate", v.Children)
	}
	if agg := v.Children[1]; !agg.Aggregate || agg.Skip != 1 || agg.Count != 2 {
		t.Fatalf("aggregate = %+v, want Aggregate true, Skip 1, Count 2", agg)
	}
	if v.Children[0].Skip != 0 {
		t.Fatalf("non-aggregate child Skip = %d, want 0", v.Children[0].Skip)
	}
}

func TestChildren(t *testing.T) {
	a, _, _, st := scanDone(t, Deps{})
	es, err := a.Children(st.RootID, 0, 10)
	if err != nil {
		t.Fatalf("Children(root, 0, 10): %v", err)
	}
	want := []struct {
		name  string
		size  int64
		isDir bool
	}{{"b", 1000 * unit, true}, {"a", 300 * unit, true}, {"r.txt", 7 * unit, false}}
	if len(es) != len(want) {
		t.Fatalf("Children(root, 0, 10) = %+v, want b, a, r.txt", es)
	}
	for i, w := range want {
		e := es[i]
		if e.Name != w.name || e.RelPath != w.name || e.Size != w.size || e.IsDir != w.isDir {
			t.Errorf("Children(root, 0, 10)[%d] = %+v, want Name/RelPath %q Size %d IsDir %v", i, e, w.name, w.size, w.isDir)
		}
		if e.Apparent != w.size {
			t.Errorf("Children(root, 0, 10)[%d] %q Apparent = %d, want %d", i, e.Name, e.Apparent, w.size)
		}
	}

	if es, err := a.Children(st.RootID, 1, 1); err != nil || len(es) != 1 || es[0].Name != "a" {
		t.Errorf("Children(root, 1, 1) = %+v, %v, want [a]", es, err)
	}
	if es, err := a.Children(st.RootID, -1, 1); err != nil || len(es) != 1 || es[0].Name != "b" {
		t.Errorf("Children(root, -1, 1) = %+v, %v, want [b]", es, err)
	}
	if es, err := a.Children(st.RootID, 0, 5000); err != nil || len(es) != 3 {
		t.Errorf("Children(root, 0, 5000) = %+v, %v, want 3 entries", es, err)
	}
	f := child(t, a, st.RootID, "r.txt")
	for _, c := range []struct {
		id            int32
		offset, limit int
	}{{st.RootID, 0, 0}, {st.RootID, 99, 10}, {f.ID, 0, 10}} {
		if es, err := a.Children(c.id, c.offset, c.limit); err != nil || es == nil || len(es) != 0 {
			t.Errorf("Children(%d, %d, %d) = %#v, %v, want non-nil empty slice", c.id, c.offset, c.limit, es, err)
		}
	}

	for _, id := range []int32{-1, 1 << 30} {
		if _, err := a.Children(id, 0, 10); !errors.Is(err, ErrInvalidID) {
			t.Errorf("Children(%d) error = %v, want ErrInvalidID", id, err)
		}
	}

	idle := New(newFakeRuntime(), Deps{})
	_, subErr := idle.Subtree(0, 1, 10, 0)
	if _, err := idle.Children(0, 0, 10); !errors.Is(err, ErrNoTree) || !errors.Is(subErr, ErrNoTree) {
		t.Errorf("Children without tree error = %v, want ErrNoTree like Subtree (%v)", err, subErr)
	}
}

func TestChildrenClampsLimit(t *testing.T) {
	root := t.TempDir()
	for i := 0; i < 1005; i++ {
		writeFile(t, filepath.Join(root, fmt.Sprintf("f%04d", i)), i%7)
	}
	a := New(newFakeRuntime(), Deps{})
	a.tree = fixtureTree(t, root)
	id := int32(a.tree.Root())
	for _, c := range []struct{ offset, limit, want int }{{0, 5000, 1000}, {1000, 5000, 5}, {0, 1000, 1000}, {0, 999, 999}} {
		if es, err := a.Children(id, c.offset, c.limit); err != nil || len(es) != c.want {
			t.Errorf("Children(root, %d, %d) = %d entries, %v, want %d", c.offset, c.limit, len(es), err, c.want)
		}
	}
}

func TestCrumbsRootIsAbsolutePath(t *testing.T) {
	a, _, root, st := scanDone(t, Deps{})
	cs, err := a.Crumbs(st.RootID)
	if err != nil || len(cs) != 1 || cs[0].Name != root {
		t.Fatalf("Crumbs(root) = %+v, %v, want one crumb named %q", cs, err, root)
	}
	if _, err := a.Crumbs(-1); !errors.Is(err, ErrInvalidID) {
		t.Errorf("Crumbs(-1) error = %v, want ErrInvalidID", err)
	}
}

func TestTopFilesOrdered(t *testing.T) {
	a, _, _, _ := scanDone(t, Deps{})
	es, err := a.TopFiles(2)
	if err != nil || len(es) != 2 {
		t.Fatalf("TopFiles(2) = %+v, %v, want 2 entries", es, err)
	}
	if es[0].Name != "f3" || es[0].Size != 1000*unit || es[1].Name != "f2" || es[1].Size != 200*unit {
		t.Fatalf("TopFiles(2) = %+v, want f3(%d), f2(%d)", es, 1000*unit, 200*unit)
	}
	if es[0].Apparent != es[0].Size || es[1].Apparent != es[1].Size {
		t.Errorf("TopFiles(2) Apparent = %d, %d, want Size %d, %d", es[0].Apparent, es[1].Apparent, es[0].Size, es[1].Size)
	}
}

func TestTopDirsExcludesRoot(t *testing.T) {
	a, _, _, st := scanDone(t, Deps{})
	es, err := a.TopDirs(5)
	if err != nil {
		t.Fatalf("TopDirs: %v", err)
	}
	for _, e := range es {
		if e.ID == st.RootID {
			t.Fatalf("TopDirs(5) contains root: %+v", es)
		}
	}
	if len(es) != 2 || es[0].Name != "b" || es[1].Name != "a" {
		t.Fatalf("TopDirs(5) = %+v, want b, a", es)
	}
}

func TestExtensionsTotalIsRootSize(t *testing.T) {
	a, _, _, st := scanDone(t, Deps{})
	r, err := a.Extensions(1)
	if err != nil || r.Total != st.Summary.Size {
		t.Fatalf("Extensions(1) = %+v, %v, want Total %d", r, err, st.Summary.Size)
	}
}

func TestScanErrorsEmpty(t *testing.T) {
	a, _, _, _ := scanDone(t, Deps{})
	es, err := a.ScanErrors(0)
	if err != nil || len(es) != 0 {
		t.Fatalf("ScanErrors(0) = %+v, %v, want empty", es, err)
	}
}

func TestNodePathOfFile(t *testing.T) {
	a, _, root, st := scanDone(t, Deps{})
	f := child(t, a, st.RootID, "r.txt")
	got, err := a.NodePath(f.ID)
	if want := filepath.Join(root, "r.txt"); err != nil || got != want {
		t.Fatalf("NodePath(r.txt) = %q, %v, want %q", got, err, want)
	}
	if _, err := a.NodePath(-1); !errors.Is(err, ErrInvalidID) {
		t.Errorf("NodePath(-1) error = %v, want ErrInvalidID", err)
	}
}

func TestRevealPassesNodePath(t *testing.T) {
	var mu sync.Mutex
	var got []string
	a, _, root, st := scanDone(t, Deps{Reveal: func(_ context.Context, p string) error {
		mu.Lock()
		defer mu.Unlock()
		got = append(got, p)
		return nil
	}})
	f := child(t, a, st.RootID, "r.txt")
	if err := a.Reveal(f.ID); err != nil {
		t.Fatalf("Reveal: %v", err)
	}
	mu.Lock()
	defer mu.Unlock()
	if want := []string{filepath.Join(root, "r.txt")}; !slices.Equal(got, want) {
		t.Fatalf("Deps.Reveal paths = %v, want %v", got, want)
	}
	if err := a.Reveal(-1); !errors.Is(err, ErrInvalidID) {
		t.Errorf("Reveal(-1) error = %v, want ErrInvalidID", err)
	}
}

func TestCopyPathSetsClipboard(t *testing.T) {
	a, rt, root, st := scanDone(t, Deps{})
	f := child(t, a, st.RootID, "r.txt")
	if err := a.CopyPath(f.ID); err != nil {
		t.Fatalf("CopyPath: %v", err)
	}
	rt.mu.Lock()
	clips := slices.Clone(rt.clips)
	rt.mu.Unlock()
	if want := []string{filepath.Join(root, "r.txt")}; !slices.Equal(clips, want) {
		t.Fatalf("clipboard = %v, want %v", clips, want)
	}
	if err := a.CopyPath(-1); !errors.Is(err, ErrInvalidID) {
		t.Errorf("CopyPath(-1) error = %v, want ErrInvalidID", err)
	}
}

func TestDeleteRootRefused(t *testing.T) {
	calls := 0
	a, _, _, st := scanDone(t, Deps{Delete: func(fsops.Guard, string) error { calls++; return nil }})
	if _, err := a.Delete(st.RootID); !errors.Is(err, ErrRootDelete) {
		t.Fatalf("Delete(root) error = %v, want ErrRootDelete", err)
	}
	if calls != 0 {
		t.Fatalf("Deps.Delete called %d times for root, want 0", calls)
	}
}

func TestDeleteInvalidID(t *testing.T) {
	a, _, _, _ := scanDone(t, Deps{})
	if _, err := a.Delete(-1); !errors.Is(err, ErrInvalidID) {
		t.Fatalf("Delete(-1) error = %v, want ErrInvalidID", err)
	}
}

func TestDeleteFileUpdatesTree(t *testing.T) {
	a, rt, root, st := scanDone(t, Deps{})
	f := child(t, a, st.RootID, "r.txt")
	before, err := a.Subtree(st.RootID, 0, 0, 0)
	if err != nil {
		t.Fatalf("Subtree(root) before delete: %v", err)
	}
	res, err := a.Delete(f.ID)
	if err != nil {
		t.Fatalf("Delete(r.txt): %v", err)
	}
	if want := (DeleteResult{ParentID: st.RootID, RemovedSize: 7 * unit, RemovedFiles: 1}); res != want {
		t.Fatalf("DeleteResult = %+v, want %+v", res, want)
	}
	if _, err := os.Lstat(filepath.Join(root, "r.txt")); !os.IsNotExist(err) {
		t.Fatalf("r.txt still on disk: %v", err)
	}
	if got := a.State().Summary; got != st.Summary {
		t.Fatalf("Summary changed after delete: %+v, want %+v", got, st.Summary)
	}
	v, err := a.Subtree(st.RootID, 1, 10, 0)
	if err != nil || v.Size != st.Summary.Size-7*unit {
		t.Fatalf("Subtree(root) Size = %d, %v, want %d", v.Size, err, st.Summary.Size-7*unit)
	}
	if v.Apparent != before.Apparent-7*unit {
		t.Errorf("Subtree(root) Apparent = %d, want %d (%d before deleting r.txt)", v.Apparent, before.Apparent-7*unit, before.Apparent)
	}
	if _, err := a.NodePath(f.ID); !errors.Is(err, ErrInvalidID) {
		t.Fatalf("NodePath(deleted) error = %v, want ErrInvalidID", err)
	}
	evs := rt.eventsNamed(EventTreeChanged)
	if len(evs) != 1 {
		t.Fatalf("tree:changed emitted %d times, want 1", len(evs))
	}
	if want := (TreeChangedEvent{RemovedID: f.ID, ParentID: st.RootID}); evs[0].Data != want {
		t.Fatalf("tree:changed data = %#v, want %#v", evs[0].Data, want)
	}
}

func TestDeleteDirCountsDescendants(t *testing.T) {
	a, _, _, st := scanDone(t, Deps{})
	d := child(t, a, st.RootID, "a")
	res, err := a.Delete(d.ID)
	if err != nil {
		t.Fatalf("Delete(a): %v", err)
	}
	// a has no subdirectories, so DirCount+1 == 1.
	want := DeleteResult{ParentID: st.RootID, RemovedSize: 300 * unit, RemovedFiles: d.FileCount, RemovedDirs: 1}
	if res != want || d.FileCount != 2 {
		t.Fatalf("DeleteResult = %+v (FileCount %d), want %+v", res, d.FileCount, want)
	}
}

func TestDeleteFailureMarksStale(t *testing.T) {
	boom := errors.New("boom")
	a, rt, _, st := scanDone(t, Deps{Delete: func(fsops.Guard, string) error { return boom }})
	f := child(t, a, st.RootID, "r.txt")
	if _, err := a.Delete(f.ID); !errors.Is(err, boom) {
		t.Fatalf("Delete error = %v, want %v", err, boom)
	}
	if !a.State().TreeStale {
		t.Fatal("TreeStale = false after failed delete, want true")
	}
	if n := len(rt.eventsNamed(EventTreeChanged)); n != 0 {
		t.Fatalf("tree:changed emitted %d times after failed delete, want 0", n)
	}
	v, err := a.Subtree(st.RootID, 1, 10, 0)
	if err != nil || v.Size != st.Summary.Size {
		t.Fatalf("Subtree(root) Size = %d, %v, want unchanged %d", v.Size, err, st.Summary.Size)
	}
	if _, err := a.NodePath(f.ID); err != nil {
		t.Fatalf("NodePath(not deleted) error = %v, want nil", err)
	}
}

func TestDeleteRefusedKeepsTreeFresh(t *testing.T) {
	refused := fmt.Errorf("%w: path is the home directory", fsops.ErrRefused)
	a, rt, _, st := scanDone(t, Deps{Delete: func(fsops.Guard, string) error { return refused }})
	f := child(t, a, st.RootID, "r.txt")
	if _, err := a.Delete(f.ID); !errors.Is(err, fsops.ErrRefused) {
		t.Fatalf("Delete error = %v, want fsops.ErrRefused", err)
	}
	if a.State().TreeStale {
		t.Fatal("TreeStale = true after refused delete, want false")
	}
	if n := len(rt.eventsNamed(EventTreeChanged)); n != 0 {
		t.Fatalf("tree:changed emitted %d times after refused delete, want 0", n)
	}
	v, err := a.Subtree(st.RootID, 1, 10, 0)
	if err != nil || v.Size != st.Summary.Size {
		t.Fatalf("Subtree(root) Size = %d, %v, want unchanged %d", v.Size, err, st.Summary.Size)
	}
	if _, err := a.NodePath(f.ID); err != nil {
		t.Fatalf("NodePath(refused) error = %v, want nil", err)
	}
}

func TestDeleteGuardProtectsHomeAndMountpoints(t *testing.T) {
	// Native separators: Guard.Protected is only ever asked about clean paths.
	home, mount := filepath.FromSlash("/home/fake"), filepath.FromSlash("/mnt/fake")
	var g fsops.Guard
	var gotPath string
	a, _, root, st := scanDone(t, Deps{
		Home:        home,
		Mountpoints: func(context.Context) ([]string, error) { return []string{mount}, nil },
		Delete:      func(guard fsops.Guard, p string) error { g, gotPath = guard, p; return nil },
	})
	f := child(t, a, st.RootID, "r.txt")
	if _, err := a.Delete(f.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if g.Protected == nil {
		t.Fatal("Guard.Protected = nil, want a predicate protecting Home and mount points")
	}
	want := filepath.Join(root, "r.txt")
	if !g.Protected(home) || !g.Protected(mount) || g.Protected(want) {
		t.Fatalf("Guard.Protected(%q, %q, %q) = %v, %v, %v; want true, true, false",
			home, mount, want, g.Protected(home), g.Protected(mount), g.Protected(want))
	}
	if gotPath != want {
		t.Fatalf("Deps.Delete path = %q, want %q", gotPath, want)
	}
}

func TestStateConcurrentDuringScan(t *testing.T) {
	a, _, _ := startBlocked(t, nil)
	var wg sync.WaitGroup
	for range 50 {
		wg.Go(func() { _ = a.State() })
	}
	wg.Wait()
}
