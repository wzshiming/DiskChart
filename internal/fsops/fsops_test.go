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

func TestDeleteRefusesMountpoint(t *testing.T) {
	mp := filepath.Join(t.TempDir(), "mnt")
	mustMkdir(t, mp)
	mustWrite(t, filepath.Join(mp, "f"))
	assertRefused(t, Guard{Mountpoints: []string{mp}}, mp)
	assertExists(t, mp)
}

func TestDeleteRefusesHome(t *testing.T) {
	h := filepath.Join(t.TempDir(), "home")
	mustMkdir(t, h)
	mustWrite(t, filepath.Join(h, "f"))
	assertRefused(t, Guard{Home: h}, h)
	assertExists(t, h)
}

func TestDeleteRefusesHomeWithTrailingSeparator(t *testing.T) {
	h := filepath.Join(t.TempDir(), "home")
	mustMkdir(t, h)
	mustWrite(t, filepath.Join(h, "f"))
	assertRefused(t, Guard{Home: h + string(filepath.Separator)}, h)
	assertExists(t, h)
}

func TestDeleteRefusesMountpointWithTrailingSeparator(t *testing.T) {
	mp := filepath.Join(t.TempDir(), "mnt")
	mustMkdir(t, mp)
	mustWrite(t, filepath.Join(mp, "f"))
	assertRefused(t, Guard{Mountpoints: []string{mp + string(filepath.Separator)}}, mp)
	assertExists(t, mp)
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

func TestDeleteRemovesDirectoryTree(t *testing.T) {
	dir := t.TempDir()
	tree := filepath.Join(dir, "tree")
	mustMkdir(t, filepath.Join(tree, "sub"))
	mustWrite(t, filepath.Join(tree, "a.txt"))
	mustWrite(t, filepath.Join(tree, "sub", "b.txt"))
	g := Guard{Mountpoints: []string{filepath.Join(dir, "other")}, Home: ""}
	if err := g.Delete(tree); err != nil {
		t.Fatalf("Delete(tree) = %v", err)
	}
	if _, err := os.Lstat(tree); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("tree still present after Delete: %v", err)
	}
	assertExists(t, dir)
}
