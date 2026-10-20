package fsops

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

func mustMkdir(t *testing.T, p string) {
	t.Helper()
	if err := os.MkdirAll(p, 0o755); err != nil {
		t.Fatal(err)
	}
}

func mustWrite(t *testing.T, p string) {
	t.Helper()
	if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func assertExists(t *testing.T, p string) {
	t.Helper()
	if _, err := os.Lstat(p); err != nil {
		t.Fatalf("%s should still exist: %v", p, err)
	}
}

func assertRefused(t *testing.T, g Guard, p string) {
	t.Helper()
	err := g.Delete(p)
	if !errors.Is(err, ErrRefused) {
		t.Fatalf("Delete(%q) = %v, want ErrRefused", p, err)
	}
}

func TestDeleteRefusesRelativePath(t *testing.T) {
	assertRefused(t, Guard{}, filepath.Join("tmp", "x"))
}

func TestDeleteRefusesPathWithDotDot(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a")
	mustMkdir(t, a)
	sep := string(filepath.Separator)
	assertRefused(t, Guard{}, a+sep+".."+sep+"a")
	assertExists(t, a)
}

func TestDeleteRefusesPathWithTrailingSeparator(t *testing.T) {
	dir := t.TempDir()
	b := filepath.Join(dir, "b")
	mustMkdir(t, b)
	assertRefused(t, Guard{}, b+string(filepath.Separator))
	assertExists(t, b)
}

func TestDeleteRefusesVolumeRoot(t *testing.T) {
	root := filepath.VolumeName(t.TempDir()) + string(filepath.Separator)
	assertRefused(t, Guard{}, root)
}

func TestDeleteMalformedPathSkipsProtected(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a")
	mustMkdir(t, a)
	sep := string(filepath.Separator)
	for _, tc := range []struct{ name, path string }{
		{"relative", filepath.Join("tmp", "x")},
		{"dot-dot", a + sep + ".." + sep + "a"},
		{"trailing separator", a + sep},
		{"volume root", filepath.VolumeName(dir) + sep},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			assertRefused(t, Guard{Protected: func(string) bool { calls++; return true }}, tc.path)
			if calls != 0 {
				t.Fatalf("Protected called %d times for %q, want 0", calls, tc.path)
			}
		})
	}
}

func TestDeleteRefusesProtectedPath(t *testing.T) {
	d := filepath.Join(t.TempDir(), "d")
	f := filepath.Join(d, "f")
	mustMkdir(t, d)
	mustWrite(t, f)
	var got []string
	assertRefused(t, Guard{Protected: func(p string) bool { got = append(got, p); return true }}, d)
	if len(got) != 1 || got[0] != d {
		t.Fatalf("Protected called with %q, want exactly once with %q", got, d)
	}
	assertExists(t, d)
	assertExists(t, f)
}

func TestDeleteProtectedMissingPathStillRefused(t *testing.T) {
	p := filepath.Join(t.TempDir(), "missing")
	err := Guard{Protected: func(string) bool { return true }}.Delete(p)
	if !errors.Is(err, ErrRefused) || errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("Delete(protected missing) = %v, want ErrRefused and not fs.ErrNotExist", err)
	}
}

func TestDeleteMissingPathReturnsNotExist(t *testing.T) {
	p := filepath.Join(t.TempDir(), "missing")
	err := Guard{}.Delete(p)
	if !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("Delete(missing) = %v, want fs.ErrNotExist", err)
	}
	if errors.Is(err, ErrRefused) {
		t.Fatalf("Delete(missing) = %v, must not be ErrRefused", err)
	}
}

func TestDeleteRemovesFile(t *testing.T) {
	f := filepath.Join(t.TempDir(), "f.txt")
	mustWrite(t, f)
	if err := (Guard{}).Delete(f); err != nil {
		t.Fatalf("Delete(file) = %v", err)
	}
	if _, err := os.Lstat(f); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("file still present after Delete: %v", err)
	}
}

func TestDeleteUnprotectedPathRemoved(t *testing.T) {
	f := filepath.Join(t.TempDir(), "f.txt")
	mustWrite(t, f)
	calls := 0
	g := Guard{Protected: func(string) bool { calls++; return false }}
	if err := g.Delete(f); err != nil {
		t.Fatalf("Delete(unprotected) = %v", err)
	}
	if calls != 1 {
		t.Fatalf("Protected called %d times, want 1", calls)
	}
	if _, err := os.Lstat(f); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("file still present after Delete: %v", err)
	}
}

func TestDeleteRemovesDirectoryTree(t *testing.T) {
	dir := t.TempDir()
	tree := filepath.Join(dir, "tree")
	other := filepath.Join(dir, "other")
	mustMkdir(t, filepath.Join(tree, "sub"))
	mustMkdir(t, other)
	mustWrite(t, filepath.Join(tree, "a.txt"))
	mustWrite(t, filepath.Join(tree, "sub", "b.txt"))
	g := Guard{Protected: func(p string) bool { return p == other }}
	if err := g.Delete(tree); err != nil {
		t.Fatalf("Delete(tree) = %v", err)
	}
	if _, err := os.Lstat(tree); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("tree still present after Delete: %v", err)
	}
	assertExists(t, other)
}
