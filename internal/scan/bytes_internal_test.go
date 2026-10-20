//go:build unix

package scan

import (
	"math/rand/v2"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"testing"
	"time"
)

func TestProgressBytesCountAllocatedSize(t *testing.T) {
	const sparseLen, fullLen = 1 << 20, 4096
	dir := t.TempDir()
	sparse := filepath.Join(dir, "sparse")
	f, err := os.Create(sparse)
	if err != nil {
		t.Fatal(err)
	}
	err = f.Truncate(sparseLen)
	f.Close()
	if err != nil {
		t.Fatal(err)
	}
	fi, err := os.Lstat(sparse)
	if err != nil {
		t.Fatal(err)
	}
	sparseAlloc := int64(fi.Sys().(*syscall.Stat_t).Blocks) * 512
	if sparseAlloc >= sparseLen {
		t.Skipf("file system allocated %d bytes for a %d-byte hole", sparseAlloc, sparseLen)
	}
	// Fixed-seed random bytes, which no file system can compress.
	full := make([]byte, fullLen)
	rand.NewChaCha8([32]byte{}).Read(full)
	if err := os.WriteFile(filepath.Join(dir, "full"), full, 0o644); err != nil {
		t.Fatal(err)
	}

	// Intermediate reports are timer-driven, so read the counter they report after one readDir step.
	s := &scanner{t: &Tree{nodes: []node{{parent: None, name: dir, isDir: true}}}}
	s.cond = sync.NewCond(&s.mu)
	s.readDir(job{0, dir})

	got := s.snapshot(time.Now()).Bytes
	if want := sparseAlloc + fullLen; got != want {
		t.Errorf("Progress.Bytes = %d, want allocated total %d", got, want)
	}
	if got == sparseLen+fullLen {
		t.Errorf("Progress.Bytes = %d, the apparent total; want allocated bytes", got)
	}
}
