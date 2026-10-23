package app

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"os"
	"os/user"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/wzshiming/DiskChart/internal/advice"
	"github.com/wzshiming/DiskChart/internal/fsops"
	"github.com/wzshiming/DiskChart/internal/scan"
	"github.com/wzshiming/DiskChart/internal/userrules"
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
	a, rt, st := advScanRoot(t, root, deps)
	return a, rt, root, st
}

// advScanRoot runs the default (real) scan over root with deps and waits for it to finish.
func advScanRoot(t *testing.T, root string, deps Deps) (*App, *fakeRuntime, ScanState) {
	t.Helper()
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
	return a, rt, st
}

// adviceFixture is fixture plus home/node_modules/f=50 (Size 1357, FileCount 5, DirCount 4); Children(root) is b, a, home, r.txt.
func adviceFixture(t *testing.T) string {
	t.Helper()
	root := fixture(t)
	writeFile(t, filepath.Join(root, "home", "node_modules", "f"), 50)
	return root
}

// advRules compiles only the common table around home, so a fixture under the host's temp dir is never
// classified by the host's own rules: home is protected, home/node_modules and its contents are recommended.
func advRules(home string, u advice.UserRules) *advice.Rules {
	return advice.New(advice.Config{GOOS: "test", Home: home, Getenv: func(string) string { return "" }, User: u})
}

// advHermetic is a Deps.Rules that ignores the host config and builds advRules(home, u).
func advHermetic(home string, u advice.UserRules) func(advice.Config) *advice.Rules {
	return func(advice.Config) *advice.Rules { return advRules(home, u) }
}

// advDone scans a fresh adviceFixture under hermetic rules without user rules.
func advDone(t *testing.T) (*App, *fakeRuntime, string, ScanState) {
	t.Helper()
	root := adviceFixture(t)
	a, rt, st := advScanRoot(t, root, Deps{Rules: advHermetic(filepath.Join(root, "home"), advice.UserRules{})})
	return a, rt, root, st
}

// adv is the advice pair carried by Entry and View.
type adv struct{ Advice, Reason string }

var (
	advNone        = adv{}
	advRoot        = adv{"protected", "root"}
	advHome        = adv{"protected", "home"}
	advRebuildable = adv{"recommended", "rebuildable"}
)

func advOfEntry(e Entry) adv { return adv{e.Advice, e.Reason} }

func advOfView(v View) adv { return adv{v.Advice, v.Reason} }

// advEntry returns the entry of es named name, failing the test when there is none.
func advEntry(t *testing.T, what string, es []Entry, name string) Entry {
	t.Helper()
	for _, e := range es {
		if e.Name == name {
			return e
		}
	}
	t.Fatalf("%s = %+v, want an entry named %q", what, es, name)
	return Entry{}
}

// advView returns the non-aggregate view of vs named name, failing the test when there is none.
func advView(t *testing.T, what string, vs []View, name string) View {
	t.Helper()
	for _, v := range vs {
		if v.Name == name && !v.Aggregate {
			return v
		}
	}
	t.Fatalf("%s = %+v, want a child named %q", what, vs, name)
	return View{}
}

// advWalk returns every entry beneath id, depth-first in display order.
func advWalk(t *testing.T, a *App, id int32) []Entry {
	t.Helper()
	es, err := a.Children(id, 0, maxChildrenPage)
	if err != nil {
		t.Fatalf("Children(%d): %v", id, err)
	}
	var out []Entry
	for _, e := range es {
		out = append(out, e)
		if e.IsDir {
			out = append(out, advWalk(t, a, e.ID)...)
		}
	}
	return out
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
	// Host-absolute paths: the rules drop anchors that are not absolute here, and Guard.Protected is only ever asked about clean paths.
	host := t.TempDir()
	home, mount := filepath.Join(host, "home"), filepath.Join(host, "mnt")
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

func TestDeleteGuardCleansHomeAndMountpoints(t *testing.T) {
	// Home and mount points may carry a trailing separator; the guard must match their clean forms.
	host, sep := t.TempDir(), string(filepath.Separator)
	home := filepath.Join(host, "home") + sep
	mounts := []string{filepath.Join(host, "mnt") + sep, filepath.Join(host, "clean")}
	var g fsops.Guard
	a, _, root, st := scanDone(t, Deps{
		Home:        home,
		Mountpoints: func(context.Context) ([]string, error) { return mounts, nil },
		Delete:      func(guard fsops.Guard, _ string) error { g = guard; return nil },
	})
	f := child(t, a, st.RootID, "r.txt")
	if _, err := a.Delete(f.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if g.Protected == nil {
		t.Fatal("Guard.Protected = nil, want a predicate protecting Home and mount points")
	}
	for _, tc := range []struct {
		path string
		want bool
	}{
		{filepath.Join(host, "home"), true},
		{filepath.Join(host, "mnt"), true},
		{filepath.Join(host, "clean"), true},
		{filepath.Join(root, "r.txt"), false},
	} {
		if got := g.Protected(tc.path); got != tc.want {
			t.Errorf("Guard.Protected(%q) = %v, want %v with Home %q and mount points %q", tc.path, got, tc.want, home, mounts)
		}
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

func TestAdviceInChildren(t *testing.T) {
	a, _, _, st := advDone(t)
	es, err := a.Children(st.RootID, 0, 10)
	if err != nil {
		t.Fatalf("Children(root): %v", err)
	}
	want := map[string]adv{"b": advNone, "a": advNone, "home": advHome, "r.txt": advNone}
	if len(es) != len(want) {
		t.Fatalf("Children(root) = %+v, want b, a, home, r.txt", es)
	}
	for _, e := range es {
		if got := advOfEntry(e); got != want[e.Name] {
			t.Errorf("Children(root) %q advice = %+v, want %+v", e.Name, got, want[e.Name])
		}
	}
	home := advEntry(t, "Children(root)", es, "home")
	es, err = a.Children(home.ID, 0, 10)
	if err != nil {
		t.Fatalf("Children(home): %v", err)
	}
	nm := advEntry(t, "Children(home)", es, "node_modules")
	if got := advOfEntry(nm); got != advRebuildable {
		t.Errorf("Children(home) node_modules advice = %+v, want %+v", got, advRebuildable)
	}
	es, err = a.Children(nm.ID, 0, 10)
	if err != nil {
		t.Fatalf("Children(node_modules): %v", err)
	}
	if f := advEntry(t, "Children(node_modules)", es, "f"); advOfEntry(f) != advRebuildable {
		t.Errorf("Children(node_modules) f advice = %+v, want %+v", advOfEntry(f), advRebuildable)
	}
}

func TestAdviceInSubtree(t *testing.T) {
	a, _, _, st := advDone(t)
	v, err := a.Subtree(st.RootID, 2, 10, 0)
	if err != nil {
		t.Fatalf("Subtree(root, 2, 10, 0): %v", err)
	}
	if got := advOfView(v); got != advRoot {
		t.Errorf("Subtree(root) advice = %+v, want %+v", got, advRoot)
	}
	home := advView(t, "Subtree(root).Children", v.Children, "home")
	if got := advOfView(home); got != advHome {
		t.Errorf("Subtree(root) home advice = %+v, want %+v", got, advHome)
	}
	nm := advView(t, "Subtree(root) home.Children", home.Children, "node_modules")
	if got := advOfView(nm); got != advRebuildable {
		t.Errorf("Subtree(root) home/node_modules advice = %+v, want %+v", got, advRebuildable)
	}
	if hv, err := a.Subtree(home.ID, 0, 0, 0); err != nil || advOfView(hv) != advHome {
		t.Errorf("Subtree(home, 0, 0, 0) advice = %+v, %v, want %+v", advOfView(hv), err, advHome)
	}
	v, err = a.Subtree(st.RootID, 1, 2, 0)
	if err != nil {
		t.Fatalf("Subtree(root, 1, 2, 0): %v", err)
	}
	i := slices.IndexFunc(v.Children, func(c View) bool { return c.Aggregate })
	if i < 0 {
		t.Fatalf("Subtree(root, 1, 2, 0).Children = %+v, want an aggregate", v.Children)
	}
	if got := advOfView(v.Children[i]); got != advNone {
		t.Errorf("aggregate advice = %+v, want empty Advice and Reason", got)
	}
}

func TestAdviceInTopLists(t *testing.T) {
	a, _, _, _ := advDone(t)
	fs, err := a.TopFiles(10)
	if err != nil {
		t.Fatalf("TopFiles(10): %v", err)
	}
	if f := advEntry(t, "TopFiles(10)", fs, "f"); advOfEntry(f) != advRebuildable {
		t.Errorf("TopFiles(10) f advice = %+v, want %+v", advOfEntry(f), advRebuildable)
	}
	if f3 := advEntry(t, "TopFiles(10)", fs, "f3"); advOfEntry(f3) != advNone {
		t.Errorf("TopFiles(10) f3 advice = %+v, want empty", advOfEntry(f3))
	}
	ds, err := a.TopDirs(10)
	if err != nil {
		t.Fatalf("TopDirs(10): %v", err)
	}
	if nm := advEntry(t, "TopDirs(10)", ds, "node_modules"); advOfEntry(nm) != advRebuildable {
		t.Errorf("TopDirs(10) node_modules advice = %+v, want %+v", advOfEntry(nm), advRebuildable)
	}
}

func TestAdviceConsistentAcrossEndpoints(t *testing.T) {
	a, _, _, st := advDone(t)
	home := child(t, a, st.RootID, "home")
	es, err := a.Children(home.ID, 0, 10)
	if err != nil {
		t.Fatalf("Children(home): %v", err)
	}
	ds, err := a.TopDirs(10)
	if err != nil {
		t.Fatalf("TopDirs(10): %v", err)
	}
	r, err := a.Cleanup(10)
	if err != nil {
		t.Fatalf("Cleanup(10): %v", err)
	}
	got := map[string]adv{
		"Children": advOfEntry(advEntry(t, "Children(home)", es, "node_modules")),
		"Subtree":  advOfView(child(t, a, home.ID, "node_modules")),
		"TopDirs":  advOfEntry(advEntry(t, "TopDirs(10)", ds, "node_modules")),
		"Cleanup":  advOfEntry(advEntry(t, "Cleanup(10).Entries", r.Entries, "node_modules")),
	}
	for k, v := range got {
		if v != got["Children"] {
			t.Errorf("node_modules advice from %s = %+v, want %+v as from Children (all: %v)", k, v, got["Children"], got)
		}
	}
}

func TestDeleteProtectedRefused(t *testing.T) {
	a, rt, root, st := advDone(t)
	home := child(t, a, st.RootID, "home")
	if _, err := a.Delete(home.ID); !errors.Is(err, fsops.ErrRefused) {
		t.Fatalf("Delete(home) error = %v, want fsops.ErrRefused", err)
	}
	if a.State().TreeStale {
		t.Error("TreeStale = true after refused delete, want false")
	}
	if n := len(rt.eventsNamed(EventTreeChanged)); n != 0 {
		t.Errorf("tree:changed emitted %d times after refused delete, want 0", n)
	}
	if _, err := os.Lstat(filepath.Join(root, "home", "node_modules", "f")); err != nil {
		t.Errorf("home/node_modules/f after refused delete: %v, want still on disk", err)
	}
	if v, err := a.Subtree(st.RootID, 0, 0, 0); err != nil || v.Size != st.Summary.Size {
		t.Errorf("Subtree(root) Size = %d, %v, want unchanged %d", v.Size, err, st.Summary.Size)
	}
}

func TestProtectedEntriesNeverDeletable(t *testing.T) {
	root := adviceFixture(t)
	u := advice.UserRules{Protected: []string{filepath.Join(root, "b")}}
	a, _, st := advScanRoot(t, root, Deps{Rules: advHermetic(filepath.Join(root, "home"), u)})
	before := advWalk(t, a, st.RootID)
	var prot []Entry
	var rels []string
	for _, e := range before {
		if e.Advice == "protected" {
			prot = append(prot, e)
			rels = append(rels, e.RelPath)
		}
	}
	slices.Sort(rels)
	if want := []string{"b", filepath.Join("b", "f3"), "home"}; !slices.Equal(rels, want) {
		t.Fatalf("entries with Advice protected = %v, want %v", rels, want)
	}
	for _, e := range prot {
		if _, err := a.Delete(e.ID); !errors.Is(err, fsops.ErrRefused) {
			t.Errorf("Delete(%s) error = %v, want fsops.ErrRefused", e.RelPath, err)
		}
	}
	if _, err := a.Delete(st.RootID); !errors.Is(err, ErrRootDelete) {
		t.Errorf("Delete(root) error = %v, want ErrRootDelete", err)
	}
	after := advWalk(t, a, st.RootID)
	if v, err := a.Subtree(st.RootID, 0, 0, 0); err != nil || v.Size != st.Summary.Size || len(after) != len(before) {
		t.Errorf("after refused deletes root Size = %d with %d nodes, %v; want Size %d with %d nodes", v.Size, len(after), err, st.Summary.Size, len(before))
	}
}

func TestDeleteRecommendedSucceeds(t *testing.T) {
	a, rt, root, st := advDone(t)
	home := child(t, a, st.RootID, "home")
	nm := child(t, a, home.ID, "node_modules")
	res, err := a.Delete(nm.ID)
	if err != nil {
		t.Fatalf("Delete(node_modules): %v", err)
	}
	if want := (DeleteResult{ParentID: home.ID, RemovedSize: 50 * unit, RemovedFiles: 1, RemovedDirs: 1}); res != want {
		t.Errorf("DeleteResult = %+v, want %+v", res, want)
	}
	if _, err := os.Lstat(filepath.Join(root, "home", "node_modules")); !os.IsNotExist(err) {
		t.Errorf("home/node_modules still on disk: %v", err)
	}
	if n := len(rt.eventsNamed(EventTreeChanged)); n != 1 {
		t.Errorf("tree:changed emitted %d times, want 1", n)
	}
	r, err := a.Cleanup(10)
	if err != nil {
		t.Fatalf("Cleanup(10) after deleting node_modules: %v", err)
	}
	if r.Total != 0 || r.Count != 0 || r.Entries == nil || len(r.Entries) != 0 {
		t.Errorf("Cleanup(10) after deleting node_modules = %+v, want Total 0, Count 0, non-nil empty Entries", r)
	}
}

func TestCleanupReport(t *testing.T) {
	a, _, _, _ := advDone(t)
	r, err := a.Cleanup(10)
	if err != nil {
		t.Fatalf("Cleanup(10): %v", err)
	}
	if r.Count != 1 || r.Total != 50*unit || len(r.Entries) != 1 {
		t.Fatalf("Cleanup(10) = %+v, want Count 1, Total %d, one entry", r, 50*unit)
	}
	e, rel := r.Entries[0], filepath.Join("home", "node_modules")
	if e.Name != "node_modules" || e.RelPath != rel || !e.IsDir || e.Size != 50*unit || advOfEntry(e) != advRebuildable {
		t.Errorf("Cleanup(10).Entries[0] = %+v, want node_modules at %q, IsDir, Size %d, advice %+v", e, rel, 50*unit, advRebuildable)
	}
	for _, limit := range []int{0, -1} {
		r, err := a.Cleanup(limit)
		if err != nil || r.Entries == nil || len(r.Entries) != 0 || r.Count != 1 || r.Total != 50*unit {
			t.Errorf("Cleanup(%d) = %+v, %v, want non-nil empty Entries, Count 1, Total %d", limit, r, err, 50*unit)
		}
	}
	big, err := a.Cleanup(5000)
	if err != nil {
		t.Fatalf("Cleanup(5000): %v", err)
	}
	if capped, err := a.Cleanup(1000); err != nil || !reflect.DeepEqual(big, capped) {
		t.Errorf("Cleanup(5000) = %+v, want Cleanup(1000) = %+v, %v", big, capped, err)
	}
}

func TestCleanupTopmostAndOrdered(t *testing.T) {
	root := adviceFixture(t)
	u := advice.UserRules{Names: []string{"a"}}
	a, _, _ := advScanRoot(t, root, Deps{Rules: advHermetic(filepath.Join(root, "home"), u)})
	r, err := a.Cleanup(10)
	if err != nil {
		t.Fatalf("Cleanup(10): %v", err)
	}
	if r.Count != 2 || r.Total != 350*unit || len(r.Entries) != 2 {
		t.Fatalf("Cleanup(10) = %+v, want Count 2, Total %d, entries a and node_modules", r, 350*unit)
	}
	custom := adv{"recommended", "custom"}
	if x, y := r.Entries[0], r.Entries[1]; x.Name != "a" || advOfEntry(x) != custom || y.Name != "node_modules" || advOfEntry(y) != advRebuildable {
		t.Errorf("Cleanup(10).Entries = %+v, want a (%+v) then node_modules (%+v)", r.Entries, custom, advRebuildable)
	}
	var sum int64
	for i, e := range r.Entries {
		sum += e.Size
		for j, o := range r.Entries {
			if i != j && strings.HasPrefix(o.RelPath, e.RelPath+string(filepath.Separator)) {
				t.Errorf("Cleanup(10) reports %q beneath %q, want topmost matches only", o.RelPath, e.RelPath)
			}
		}
	}
	if sum != r.Total {
		t.Errorf("Cleanup(10) entries sum to %d, want Total %d", sum, r.Total)
	}
	if r1, err := a.Cleanup(1); err != nil || len(r1.Entries) != 1 || r1.Entries[0].Name != "a" || r1.Count != 2 || r1.Total != 350*unit {
		t.Errorf("Cleanup(1) = %+v, %v, want Entries [a], Count 2, Total %d", r1, err, 350*unit)
	}
}

// The scan root is protected even when the rules would recommend it, so Cleanup reports its contents instead.
func TestCleanupExcludesRecommendedRoot(t *testing.T) {
	home := filepath.Join(adviceFixture(t), "home")
	a, _, st := advScanRoot(t, filepath.Join(home, "node_modules"), Deps{Rules: advHermetic(home, advice.UserRules{})})
	if v, err := a.Subtree(st.RootID, 0, 0, 0); err != nil || advOfView(v) != advRoot {
		t.Errorf("Subtree(root, 0, 0, 0) advice = %+v, %v, want %+v", advOfView(v), err, advRoot)
	}
	r, err := a.Cleanup(10)
	if err != nil {
		t.Fatalf("Cleanup(10): %v", err)
	}
	if r.Count != 1 || r.Total != 50*unit || len(r.Entries) != 1 {
		t.Fatalf("Cleanup(10) over a recommended root = %+v, want Count 1, Total %d, one entry", r, 50*unit)
	}
	if e := r.Entries[0]; e.Name != "f" || e.RelPath != "f" || advOfEntry(e) != advRebuildable {
		t.Errorf("Cleanup(10).Entries[0] = %+v, want f at RelPath %q with advice %+v, not the root", e, "f", advRebuildable)
	}
	for _, e := range r.Entries {
		if e.RelPath == "." || e.Advice == "protected" || e.ID == st.RootID {
			t.Errorf("Cleanup(10) lists the scan root as %+v, want descendants only", e)
		}
	}
}

func TestCleanupWithoutTree(t *testing.T) {
	root := adviceFixture(t)
	rules := advHermetic(filepath.Join(root, "home"), advice.UserRules{})
	if _, err := New(newFakeRuntime(), Deps{Rules: rules}).Cleanup(10); !errors.Is(err, ErrNoTree) {
		t.Errorf("Cleanup(10) before any scan error = %v, want ErrNoTree", err)
	}
	tree := fixtureTree(t, root)
	rt := newFakeRuntime()
	a := New(rt, Deps{Scan: blockingScan(tree), Mountpoints: noMounts, Rules: rules})
	if err := a.StartScan(root, ScanOptions{}); err != nil {
		t.Fatalf("StartScan: %v", err)
	}
	if _, err := a.Cleanup(10); !errors.Is(err, ErrNoTree) {
		t.Errorf("Cleanup(10) while scanning error = %v, want ErrNoTree", err)
	}
	if err := a.CancelScan(); err != nil {
		t.Fatalf("CancelScan: %v", err)
	}
	if st := waitFinished(t, a, rt); st.Status != StatusCancelled {
		t.Fatalf("Status after cancel = %q, want cancelled", st.Status)
	}
	if _, err := a.Cleanup(10); err != nil {
		t.Errorf("Cleanup(10) over the cancelled scan's tree error = %v, want nil", err)
	}
}

func TestCleanupFollowsRulesRebuild(t *testing.T) {
	root := adviceFixture(t)
	var mu sync.Mutex
	var names []string
	rules := func(advice.Config) *advice.Rules {
		mu.Lock()
		defer mu.Unlock()
		return advRules(filepath.Join(root, "home"), advice.UserRules{Names: names})
	}
	a, _, st := advScanRoot(t, root, Deps{Rules: rules})
	if r, err := a.Cleanup(10); err != nil || r.Count != 1 {
		t.Fatalf("Cleanup(10) = %+v, %v, want Count 1", r, err)
	}
	mu.Lock()
	names = []string{"b"}
	mu.Unlock()
	a.mu.Lock()
	a.rebuildRules()
	a.mu.Unlock()
	if r, err := a.Cleanup(10); err != nil || r.Count != 2 || r.Total != 1050*unit {
		t.Errorf("Cleanup(10) after rebuildRules = %+v, %v, want Count 2, Total %d", r, err, 1050*unit)
	}
	es, err := a.Children(st.RootID, 0, 10)
	if err != nil {
		t.Fatalf("Children(root): %v", err)
	}
	if b, want := advEntry(t, "Children(root)", es, "b"), (adv{"recommended", "custom"}); advOfEntry(b) != want {
		t.Errorf("Children(root) b advice after rebuildRules = %+v, want %+v", advOfEntry(b), want)
	}
}

func TestCleanupCached(t *testing.T) {
	a, _, root, st := advDone(t)
	cached := func() *CleanupReport {
		a.mu.Lock()
		defer a.mu.Unlock()
		return a.cleanup
	}
	if got := cached(); got != nil {
		t.Fatalf("a.cleanup = %+v after scan, want nil before the first Cleanup", got)
	}
	if _, err := a.Cleanup(10); err != nil {
		t.Fatalf("Cleanup(10): %v", err)
	}
	first := cached()
	if first == nil {
		t.Fatal("a.cleanup = nil after Cleanup(10), want the cached report")
	}
	if _, err := a.Cleanup(5); err != nil {
		t.Fatalf("Cleanup(5): %v", err)
	}
	if got := cached(); got != first {
		t.Errorf("a.cleanup = %p after Cleanup(5), want the same report %p", got, first)
	}

	f := child(t, a, st.RootID, "r.txt")
	if _, err := a.Delete(f.ID); err != nil {
		t.Fatalf("Delete(r.txt): %v", err)
	}
	if got := cached(); got != nil {
		t.Errorf("a.cleanup = %p after a successful Delete, want nil", got)
	}

	if _, err := a.Cleanup(10); err != nil {
		t.Fatalf("Cleanup(10) after Delete: %v", err)
	}
	a.mu.Lock()
	a.rebuildRules()
	a.mu.Unlock()
	if got := cached(); got != nil {
		t.Errorf("a.cleanup = %p after rebuildRules, want nil", got)
	}

	if _, err := a.Cleanup(10); err != nil {
		t.Fatalf("Cleanup(10) after rebuildRules: %v", err)
	}
	if err := a.StartScan(root, ScanOptions{}); err != nil {
		t.Fatalf("second StartScan: %v", err)
	}
	got := cached()
	waitState(t, a)
	if got != nil {
		t.Errorf("a.cleanup = %p after StartScan, want nil", got)
	}
}

func TestRulesConfigFromHost(t *testing.T) {
	home, mount := filepath.FromSlash("/home/fake"), filepath.FromSlash("/mnt/fake")
	var mu sync.Mutex
	var cfgs []advice.Config
	scanDone(t, Deps{
		Home:        home,
		Mountpoints: func(context.Context) ([]string, error) { return []string{mount}, nil },
		Rules: func(c advice.Config) *advice.Rules {
			mu.Lock()
			cfgs = append(cfgs, c)
			mu.Unlock()
			return advice.New(c)
		},
	})
	mu.Lock()
	defer mu.Unlock()
	if len(cfgs) != 2 {
		t.Fatalf("Deps.Rules called %d times over New and one scan, want 2 (New, then StartScan)", len(cfgs))
	}
	if len(cfgs[0].Mountpoints) != 0 {
		t.Errorf("Deps.Rules config from New has Mountpoints %v, want none", cfgs[0].Mountpoints)
	}
	c := cfgs[1]
	if !slices.Equal(c.Mountpoints, []string{mount}) {
		t.Errorf("Deps.Rules config from StartScan has Mountpoints %v, want %v", c.Mountpoints, []string{mount})
	}
	tmp, err := filepath.EvalSymlinks(os.TempDir())
	if err != nil {
		tmp = filepath.Clean(os.TempDir())
	}
	uid := ""
	if u, err := user.Current(); err == nil {
		uid = u.Uid
	}
	if c.GOOS != runtime.GOOS || c.Home != home || c.TempDir != tmp || tmp == "" || c.UID != uid || c.Getenv == nil {
		t.Errorf("Deps.Rules config = %+v, want GOOS %q, Home %q, TempDir %q, UID %q and a non-nil Getenv", c, runtime.GOOS, home, tmp, uid)
	}
	if len(c.User.Protected)+len(c.User.Recommended)+len(c.User.Names) != 0 {
		t.Errorf("Deps.Rules config User = %+v, want zero UserRules", c.User)
	}
}

// TestMain points every config-dir lookup at a scratch directory, so the default
// userrules.Store that New builds without Deps.UserRulesStore never reads the developer's rules.json.
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "diskchart-app-test-")
	if err != nil {
		fmt.Fprintln(os.Stderr, "TestMain:", err)
		os.Exit(1)
	}
	for _, k := range []string{"HOME", "USERPROFILE", "XDG_CONFIG_HOME", "AppData", "LocalAppData"} {
		if err := os.Setenv(k, dir); err != nil {
			fmt.Fprintln(os.Stderr, "TestMain:", err)
			os.Exit(1)
		}
	}
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

// rulesStore returns a Store over rules.json in a fresh temp dir.
func rulesStore(t *testing.T) userrules.Store {
	t.Helper()
	return userrules.Store{File: filepath.Join(t.TempDir(), "rules.json")}
}

func readFile(t *testing.T, p string) string {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("ReadFile(%s): %v", p, err)
	}
	return string(b)
}

// appRules converts u to the DTO; both types carry the same three lists.
func appRules(u advice.UserRules) UserRules {
	return UserRules{Protected: u.Protected, Recommended: u.Recommended, Names: u.Names}
}

// noRules is the DTO of no rules as UserRules reports it: three empty, non-nil slices.
func noRules() UserRules {
	return UserRules{Protected: []string{}, Recommended: []string{}, Names: []string{}}
}

// advFromUser is a Deps.Rules that compiles the hermetic table around home with the user rules the App passes in.
func advFromUser(home string) func(advice.Config) *advice.Rules {
	return func(c advice.Config) *advice.Rules { return advRules(home, c.User) }
}

var (
	advCustomProtected   = adv{"protected", "custom"}
	advCustomRecommended = adv{"recommended", "custom"}
)

// rulesSpy records every config Deps.Rules is called with and delegates to build.
type rulesSpy struct {
	mu    sync.Mutex
	cfgs  []advice.Config
	build func(advice.Config) *advice.Rules
}

func (s *rulesSpy) rules(c advice.Config) *advice.Rules {
	s.mu.Lock()
	s.cfgs = append(s.cfgs, c)
	s.mu.Unlock()
	return s.build(c)
}

func (s *rulesSpy) configs() []advice.Config {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.cfgs)
}

// fakeStore is a UserRulesStore that loads fixed rules, counts Saves in *saves when set, and fails every Save with err.
type fakeStore struct {
	path  string
	rules advice.UserRules
	err   error
	saves *int
}

func (s fakeStore) Path() string                    { return s.path }
func (s fakeStore) Load() (advice.UserRules, error) { return s.rules, nil }
func (s fakeStore) Save(advice.UserRules) error {
	if s.saves != nil {
		*s.saves++
	}
	return s.err
}

func TestEventPayloadsIncludeRulesChanged(t *testing.T) {
	if EventRulesChanged != "rules:changed" {
		t.Errorf("EventRulesChanged = %q, want rules:changed", EventRulesChanged)
	}
	f, ok := reflect.TypeFor[EventPayloads]().FieldByName("RulesChanged")
	if !ok || f.Type != reflect.TypeFor[RulesChangedEvent]() {
		t.Fatalf("EventPayloads.RulesChanged present %v with type %v, want a RulesChangedEvent field", ok, f.Type)
	}
	if p, ok := reflect.TypeFor[RulesChangedEvent]().FieldByName("Path"); !ok || p.Type != reflect.TypeFor[string]() {
		t.Errorf("RulesChangedEvent.Path present %v with type %v, want a string field holding the rules file", ok, p.Type)
	}
}

func TestDefaultStoreUsesDefaultPath(t *testing.T) {
	t.Run("default path", func(t *testing.T) {
		p, err := userrules.DefaultPath()
		if err != nil {
			t.Skipf("DefaultPath: %v", err)
		}
		info := New(newFakeRuntime(), Deps{}).UserRules()
		if info.Path != p || info.LoadError != "" || !reflect.DeepEqual(info.Rules, noRules()) {
			t.Fatalf("UserRules() with the default store = %+v, want Path %q, no LoadError and no rules", info, p)
		}
	})
	t.Run("no config dir", func(t *testing.T) {
		for _, k := range []string{"HOME", "XDG_CONFIG_HOME", "AppData"} {
			t.Setenv(k, "")
		}
		if _, err := os.UserConfigDir(); err == nil {
			t.Skipf("clearing the environment does not make os.UserConfigDir fail on %s", runtime.GOOS)
		}
		a := New(newFakeRuntime(), Deps{})
		if info := a.UserRules(); info.Path != "" || info.LoadError != userrules.ErrNoPath.Error() {
			t.Errorf("UserRules() without a config dir = %+v, want Path \"\" and LoadError %q", info, userrules.ErrNoPath)
		}
		if err := a.SetUserRules(UserRules{Names: []string{"x"}}); !errors.Is(err, userrules.ErrNoPath) {
			t.Errorf("SetUserRules without a config dir = %v, want ErrNoPath", err)
		}
	})
}

func TestNewLoadsUserRules(t *testing.T) {
	st := rulesStore(t)
	want := advice.UserRules{Protected: []string{"~/keep"}, Recommended: []string{"~/junk"}, Names: []string{"target"}}
	if err := st.Save(want); err != nil {
		t.Fatalf("Save: %v", err)
	}
	spy := &rulesSpy{build: advice.New}
	a := New(newFakeRuntime(), Deps{UserRulesStore: st, Rules: spy.rules})
	cfgs := spy.configs()
	if len(cfgs) == 0 {
		t.Fatal("Deps.Rules not called by New")
	}
	if !reflect.DeepEqual(cfgs[0].User, want) {
		t.Errorf("first Deps.Rules config User = %+v, want the stored %+v", cfgs[0].User, want)
	}
	if got, exp := a.UserRules(), (UserRulesInfo{Rules: appRules(want), Path: st.Path()}); !reflect.DeepEqual(got, exp) {
		t.Errorf("UserRules() = %+v, want %+v", got, exp)
	}
}

func TestUserRulesEmptyNonNil(t *testing.T) {
	info := New(newFakeRuntime(), Deps{UserRulesStore: rulesStore(t)}).UserRules()
	if info.LoadError != "" || !reflect.DeepEqual(info.Rules, noRules()) {
		t.Errorf("UserRules() without a file = %#v, want three empty non-nil slices and no LoadError", info)
	}

	st := rulesStore(t)
	stored := advice.UserRules{Protected: []string{"~/keep"}, Recommended: []string{"~/junk"}, Names: []string{"target"}}
	if err := st.Save(stored); err != nil {
		t.Fatalf("Save: %v", err)
	}
	a := New(newFakeRuntime(), Deps{UserRulesStore: st})
	got := a.UserRules()
	if !reflect.DeepEqual(got.Rules, appRules(stored)) {
		t.Fatalf("UserRules().Rules = %+v, want %+v", got.Rules, appRules(stored))
	}
	got.Rules.Protected[0], got.Rules.Recommended[0], got.Rules.Names[0] = "x", "y", "z"
	if again := a.UserRules(); !reflect.DeepEqual(again.Rules, appRules(stored)) {
		t.Errorf("UserRules().Rules after writing to a previous result = %+v, want unchanged %+v", again.Rules, appRules(stored))
	}
}

func TestSetUserRulesApplies(t *testing.T) {
	root := adviceFixture(t)
	home := filepath.Join(root, "home")
	writeFile(t, filepath.Join(home, "keep", "k"), 1)
	writeFile(t, filepath.Join(home, "junk", "j"), 1)
	st := rulesStore(t)
	var mu sync.Mutex
	scans := 0
	a, rt, state := advScanRoot(t, root, Deps{
		UserRulesStore: st,
		Rules:          advFromUser(home),
		Scan: func(ctx context.Context, root string, opts scan.Options, onProgress func(scan.Progress)) (*scan.Tree, error) {
			mu.Lock()
			scans++
			mu.Unlock()
			return scan.Scan(ctx, root, opts, onProgress)
		},
	})
	keep, junk := filepath.Join(home, "keep"), filepath.Join(home, "junk")
	if err := a.SetUserRules(UserRules{Protected: []string{keep}, Recommended: []string{junk}}); err != nil {
		t.Fatalf("SetUserRules: %v", err)
	}

	homeID := child(t, a, state.RootID, "home").ID
	es, err := a.Children(homeID, 0, 10)
	if err != nil {
		t.Fatalf("Children(home): %v", err)
	}
	keepE, junkE := advEntry(t, "Children(home)", es, "keep"), advEntry(t, "Children(home)", es, "junk")
	if got := advOfEntry(keepE); got != advCustomProtected {
		t.Errorf("Children(home) keep advice = %+v, want %+v", got, advCustomProtected)
	}
	if got := advOfEntry(junkE); got != advNone {
		t.Errorf("Children(home) junk advice = %+v, want none: a recommended anchor keeps itself", got)
	}
	jes, err := a.Children(junkE.ID, 0, 10)
	if err != nil {
		t.Fatalf("Children(junk): %v", err)
	}
	if got := advOfEntry(advEntry(t, "Children(junk)", jes, "j")); got != advCustomRecommended {
		t.Errorf("Children(junk) j advice = %+v, want %+v", got, advCustomRecommended)
	}
	if r, err := a.Cleanup(10); err != nil || r.Count != 2 || r.Total != 51*unit {
		t.Errorf("Cleanup(10) = %+v, %v, want Count 2 (node_modules, junk/j) and Total %d", r, err, 51*unit)
	}
	if _, err := a.Delete(keepE.ID); !errors.Is(err, fsops.ErrRefused) {
		t.Errorf("Delete(keep) error = %v, want fsops.ErrRefused", err)
	}
	if _, err := os.Lstat(filepath.Join(keep, "k")); err != nil {
		t.Errorf("home/keep/k after refused delete: %v, want still on disk", err)
	}

	want := advice.UserRules{Protected: []string{keep}, Recommended: []string{junk}, Names: []string{}}
	scratch := rulesStore(t)
	if err := scratch.Save(want); err != nil {
		t.Fatalf("scratch Save: %v", err)
	}
	if got, exp := readFile(t, st.File), readFile(t, scratch.File); got != exp {
		t.Errorf("rules file = %q, want Store.Save's bytes %q", got, exp)
	}
	if got, exp := a.UserRules(), (UserRulesInfo{Rules: appRules(want), Path: st.Path()}); !reflect.DeepEqual(got, exp) {
		t.Errorf("UserRules() = %+v, want %+v", got, exp)
	}
	evs := rt.eventsNamed(EventRulesChanged)
	if len(evs) != 1 || evs[0].Data != (RulesChangedEvent{Path: st.Path()}) {
		t.Errorf("rules:changed events = %+v, want exactly one with payload RulesChangedEvent{Path: %q}", evs, st.Path())
	}
	if n := len(rt.eventsNamed(EventTreeChanged)); n != 0 {
		t.Errorf("tree:changed emitted %d times by SetUserRules, want 0", n)
	}
	mu.Lock()
	defer mu.Unlock()
	if scans != 1 {
		t.Errorf("Deps.Scan called %d times, want 1: new rules apply without rescanning", scans)
	}
}

func TestSetUserRulesNormalizes(t *testing.T) {
	st := rulesStore(t)
	a := New(newFakeRuntime(), Deps{UserRulesStore: st})
	abs := filepath.Join(t.TempDir(), "keep")
	in := UserRules{
		Protected:   []string{" " + abs + " ", abs, "", "~/x", "~", "~"},
		Recommended: []string{" ", "~/y", "~/y "},
		Names:       []string{"a", " a ", "b", ""},
	}
	if err := a.SetUserRules(in); err != nil {
		t.Fatalf("SetUserRules: %v", err)
	}
	want := UserRules{Protected: []string{abs, "~/x", "~"}, Recommended: []string{"~/y"}, Names: []string{"a", "b"}}
	if got, exp := a.UserRules(), (UserRulesInfo{Rules: want, Path: st.Path()}); !reflect.DeepEqual(got, exp) {
		t.Errorf("UserRules() = %+v, want normalized %+v", got, exp)
	}
	if got, err := st.Load(); err != nil || !reflect.DeepEqual(appRules(got), want) {
		t.Errorf("Store.Load() = %+v, %v, want normalized %+v", got, err, want)
	}
}

func TestSetUserRulesInvalid(t *testing.T) {
	root := adviceFixture(t)
	home := filepath.Join(root, "home")
	st := rulesStore(t)
	if err := st.Save(advice.UserRules{Names: []string{"a"}}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	spy := &rulesSpy{build: advFromUser(home)}
	a, rt, state := advScanRoot(t, root, Deps{UserRulesStore: st, Rules: spy.rules})
	file, info, calls := readFile(t, st.File), a.UserRules(), len(spy.configs())
	before, err := a.Cleanup(10)
	if err != nil || before.Count != 2 {
		t.Fatalf("Cleanup(10) before = %+v, %v, want Count 2 (a, node_modules)", before, err)
	}

	err = a.SetUserRules(UserRules{Protected: []string{"rel"}, Names: []string{"b"}})
	if !errors.Is(err, userrules.ErrInvalidRules) {
		t.Fatalf("SetUserRules(invalid) = %v, want userrules.ErrInvalidRules", err)
	}
	if got := readFile(t, st.File); got != file {
		t.Errorf("rules file after rejected SetUserRules = %q, want unchanged %q", got, file)
	}
	if got := a.UserRules(); !reflect.DeepEqual(got, info) {
		t.Errorf("UserRules() after rejected SetUserRules = %+v, want unchanged %+v", got, info)
	}
	es, err := a.Children(state.RootID, 0, 10)
	if err != nil {
		t.Fatalf("Children(root): %v", err)
	}
	if got := advOfEntry(advEntry(t, "Children(root)", es, "a")); got != advCustomRecommended {
		t.Errorf("Children(root) a advice = %+v, want still %+v", got, advCustomRecommended)
	}
	if got := advOfEntry(advEntry(t, "Children(root)", es, "b")); got != advNone {
		t.Errorf("Children(root) b advice = %+v, want none: a rejected set must not apply in part", got)
	}
	if after, err := a.Cleanup(10); err != nil || !reflect.DeepEqual(after, before) {
		t.Errorf("Cleanup(10) after rejected SetUserRules = %+v, %v, want unchanged %+v", after, err, before)
	}
	if n := len(spy.configs()); n != calls {
		t.Errorf("Deps.Rules called %d times after rejected SetUserRules, want still %d", n, calls)
	}
	if n := len(rt.eventsNamed(EventRulesChanged)); n != 0 {
		t.Errorf("rules:changed emitted %d times after rejected SetUserRules, want 0", n)
	}
}

func TestSetUserRulesSaveFails(t *testing.T) {
	boom := errors.New("boom")
	fs := fakeStore{path: filepath.Join("fake", "rules.json"), rules: advice.UserRules{Names: []string{"x"}}, err: boom}
	spy := &rulesSpy{build: advice.New}
	rt := newFakeRuntime()
	a := New(rt, Deps{UserRulesStore: fs, Rules: spy.rules})
	calls := len(spy.configs())
	want := UserRulesInfo{Rules: UserRules{Protected: []string{}, Recommended: []string{}, Names: []string{"x"}}, Path: fs.path}
	if got := a.UserRules(); !reflect.DeepEqual(got, want) {
		t.Fatalf("UserRules() = %+v, want %+v", got, want)
	}
	if err := a.SetUserRules(UserRules{Names: []string{"y"}}); !errors.Is(err, boom) {
		t.Fatalf("SetUserRules with a failing Save = %v, want %v", err, boom)
	}
	if got := a.UserRules(); !reflect.DeepEqual(got, want) {
		t.Errorf("UserRules() after failed Save = %+v, want unchanged %+v", got, want)
	}
	if n := len(spy.configs()); n != calls {
		t.Errorf("Deps.Rules called %d times after failed Save, want still %d", n, calls)
	}
	if n := len(rt.eventsNamed(EventRulesChanged)); n != 0 {
		t.Errorf("rules:changed emitted %d times after failed Save, want 0", n)
	}
}

func TestSetUserRulesRejectsBroadRecommended(t *testing.T) {
	saves := 0
	fs := fakeStore{path: filepath.Join("fake", "rules.json"), rules: advice.UserRules{Names: []string{"x"}}, saves: &saves}
	rt := newFakeRuntime()
	a := New(rt, Deps{UserRulesStore: fs})
	before := a.UserRules()
	err := a.SetUserRules(UserRules{Recommended: []string{"~"}})
	if !errors.Is(err, userrules.ErrInvalidRules) || !strings.Contains(err.Error(), "too broad") {
		t.Fatalf("SetUserRules(Recommended ~) = %v, want userrules.ErrInvalidRules reading too broad", err)
	}
	if saves != 0 {
		t.Errorf("Save called %d times by a rejected SetUserRules, want 0", saves)
	}
	if n := len(rt.eventsNamed(EventRulesChanged)); n != 0 {
		t.Errorf("rules:changed emitted %d times by a rejected SetUserRules, want 0", n)
	}
	if got := a.UserRules(); !reflect.DeepEqual(got, before) {
		t.Errorf("UserRules() after rejected SetUserRules = %+v, want unchanged %+v", got, before)
	}
}

// homeApp returns an App whose Deps.Home is home, over a store that only counts its Saves.
func homeApp(home string) (*App, *fakeRuntime, *int) {
	saves := 0
	rt := newFakeRuntime()
	st := fakeStore{path: filepath.Join("fake", "rules.json"), saves: &saves}
	return New(rt, Deps{UserRulesStore: st, Rules: advFromUser(home), Home: home}), rt, &saves
}

func TestSetUserRulesRejectsHomeAsRecommended(t *testing.T) {
	home := filepath.Join(adviceFixture(t), "home")
	sep := string(filepath.Separator)
	// rejected checks that a fresh App refuses r with ErrInvalidRules reading line and keeps everything as it was.
	rejected := func(t *testing.T, r UserRules, line string) error {
		t.Helper()
		a, rt, saves := homeApp(home)
		before := a.UserRules()
		if !reflect.DeepEqual(before.Rules, noRules()) {
			t.Fatalf("UserRules().Rules before SetUserRules = %+v, want none", before.Rules)
		}
		err := a.SetUserRules(r)
		if !errors.Is(err, userrules.ErrInvalidRules) || !strings.Contains(err.Error(), line) {
			t.Fatalf("SetUserRules(%+v) = %v, want userrules.ErrInvalidRules reading %q", r, err, line)
		}
		if *saves != 0 {
			t.Errorf("Save called %d times by a rejected SetUserRules, want 0", *saves)
		}
		if n := len(rt.eventsNamed(EventRulesChanged)); n != 0 {
			t.Errorf("rules:changed emitted %d times by a rejected SetUserRules, want 0", n)
		}
		if got := a.UserRules(); !reflect.DeepEqual(got, before) {
			t.Errorf("UserRules() after rejected SetUserRules = %+v, want unchanged %+v", got, before)
		}
		return err
	}

	type namedPath struct{ name, path string }
	broad := []namedPath{
		{"home itself", home},
		{"trailing separator", home + sep},
		{"dot dot back to home", home + sep + "x" + sep + ".."},
		{"trailing dot", home + sep + "."},
	}
	if runtime.GOOS == "darwin" || runtime.GOOS == "windows" {
		if up := strings.ToUpper(home); up != home {
			broad = append(broad, namedPath{"other case", up})
		}
	}
	for _, tc := range broad {
		t.Run(tc.name, func(t *testing.T) {
			rejected(t, UserRules{Recommended: []string{tc.path}}, "Recommended[0]: too broad: "+tc.path)
		})
	}
	t.Run("index follows the submitted list", func(t *testing.T) {
		err := rejected(t, UserRules{Recommended: []string{filepath.Join(home, "ok"), home}}, "Recommended[1]: too broad: "+home)
		if strings.Contains(err.Error(), "Recommended[0]") {
			t.Errorf("SetUserRules error = %q, want no line for Recommended[0]: home/ok is narrower than home", err)
		}
	})
	t.Run("a directory beneath home is accepted", func(t *testing.T) {
		a, rt, saves := homeApp(home)
		if err := a.SetUserRules(UserRules{Recommended: []string{filepath.Join(home, "x")}}); err != nil {
			t.Fatalf("SetUserRules(Recommended home/x) = %v, want nil", err)
		}
		if *saves != 1 {
			t.Errorf("Save called %d times, want 1", *saves)
		}
		if n := len(rt.eventsNamed(EventRulesChanged)); n != 1 {
			t.Errorf("rules:changed emitted %d times, want 1", n)
		}
	})
}

func TestSetUserRulesHomeCheckSkippedWithoutHome(t *testing.T) {
	for _, k := range []string{"HOME", "USERPROFILE"} {
		t.Setenv(k, "")
	}
	if h, err := os.UserHomeDir(); err == nil {
		t.Skipf("clearing HOME and USERPROFILE does not make os.UserHomeDir fail on %s: %q", runtime.GOOS, h)
	}
	a, _, saves := homeApp("")
	if a.deps.Home != "" {
		t.Fatalf("Deps.Home after New = %q, want \"\" so there is no home to compare against", a.deps.Home)
	}
	dir := filepath.Join(t.TempDir(), "home")
	if err := a.SetUserRules(UserRules{Recommended: []string{dir}}); err != nil {
		t.Fatalf("SetUserRules(Recommended %q) without a home = %v, want nil", dir, err)
	}
	if *saves != 1 {
		t.Errorf("Save called %d times, want 1", *saves)
	}
}

func TestSetUserRulesAcceptsHomeAsProtected(t *testing.T) {
	home := filepath.Join(adviceFixture(t), "home")
	a, _, saves := homeApp(home)
	if err := a.SetUserRules(UserRules{Protected: []string{home}}); err != nil {
		t.Fatalf("SetUserRules(Protected home) = %v, want nil: only Recommended must stay narrower than home", err)
	}
	if *saves != 1 {
		t.Errorf("Save called %d times, want 1", *saves)
	}
}

func TestCorruptFileRecovers(t *testing.T) {
	st := rulesStore(t)
	if err := os.WriteFile(st.File, []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	spy := &rulesSpy{build: advice.New}
	rt := newFakeRuntime()
	a := New(rt, Deps{UserRulesStore: st, Rules: spy.rules})
	info := a.UserRules()
	if info.Path != st.Path() || !strings.Contains(info.LoadError, st.File) || !reflect.DeepEqual(info.Rules, noRules()) {
		t.Errorf("UserRules() over a corrupt file = %#v, want Path %q, a LoadError naming it and no rules", info, st.File)
	}
	if cfgs := spy.configs(); len(cfgs) == 0 || len(cfgs[0].User.Protected)+len(cfgs[0].User.Recommended)+len(cfgs[0].User.Names) != 0 {
		t.Errorf("Deps.Rules configs from New = %+v, want one call with zero User", cfgs)
	}

	if err := a.SetUserRules(UserRules{Names: []string{"target"}}); err != nil {
		t.Fatalf("SetUserRules after a corrupt load: %v", err)
	}
	want := UserRules{Protected: []string{}, Recommended: []string{}, Names: []string{"target"}}
	if got, exp := a.UserRules(), (UserRulesInfo{Rules: want, Path: st.Path()}); !reflect.DeepEqual(got, exp) {
		t.Errorf("UserRules() after SetUserRules = %+v, want %+v with LoadError cleared", got, exp)
	}
	if got, err := (userrules.Store{File: st.File}).Load(); err != nil || !reflect.DeepEqual(appRules(got), want) {
		t.Errorf("fresh Store.Load() = %+v, %v, want %+v", got, err, want)
	}
	if n := len(rt.eventsNamed(EventRulesChanged)); n != 1 {
		t.Errorf("rules:changed emitted %d times, want 1", n)
	}
}

func TestSetUserRulesConcurrent(t *testing.T) {
	const n = 8
	st := rulesStore(t)
	rt := newFakeRuntime()
	a := New(rt, Deps{UserRulesStore: st})
	errs := make([]error, n)
	var wg sync.WaitGroup
	for i := range n {
		wg.Go(func() { errs[i] = a.SetUserRules(UserRules{Names: []string{fmt.Sprintf("n%d", i)}}) })
	}
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Errorf("SetUserRules #%d: %v", i, err)
		}
	}
	info := a.UserRules()
	if len(info.Rules.Names) != 1 || !strings.HasPrefix(info.Rules.Names[0], "n") {
		t.Errorf("UserRules().Rules = %+v, want exactly one of the concurrently set Names", info.Rules)
	}
	if got, err := st.Load(); err != nil || !reflect.DeepEqual(appRules(got), info.Rules) {
		t.Errorf("Store.Load() = %+v, %v, want the final UserRules() %+v", got, err, info.Rules)
	}
	if evs := rt.eventsNamed(EventRulesChanged); len(evs) != n {
		t.Errorf("rules:changed emitted %d times, want %d", len(evs), n)
	}
	es, err := os.ReadDir(filepath.Dir(st.File))
	if err != nil || len(es) != 1 || es[0].Name() != "rules.json" {
		t.Errorf("rules dir holds %d entries (%v), want rules.json only", len(es), err)
	}
}
