//go:build unix

package app

import (
	"os"
	"path/filepath"
	"testing"
)

func TestScanErrorsLimit(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores directory permissions")
	}
	root := fixture(t)
	locked := filepath.Join(root, "locked")
	writeFile(t, filepath.Join(locked, "f"), 1)
	if err := os.Chmod(locked, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(locked, 0o755) })

	rt := newFakeRuntime()
	a := New(rt, Deps{Mountpoints: noMounts})
	if err := a.StartScan(root, ScanOptions{}); err != nil {
		t.Fatalf("StartScan: %v", err)
	}
	st := waitFinished(t, a, rt)
	if st.Status != StatusDone || st.Summary.Errors < 1 {
		t.Fatalf("State = %+v, want Status done with Summary.Errors >= 1", st)
	}
	all, err := a.ScanErrors(0)
	if err != nil || int64(len(all)) != st.Summary.Errors {
		t.Fatalf("ScanErrors(0) = %+v, %v, want %d errors", all, err, st.Summary.Errors)
	}
	one, err := a.ScanErrors(1)
	if err != nil || len(one) != 1 {
		t.Fatalf("ScanErrors(1) = %+v, %v, want 1 error", one, err)
	}
}
