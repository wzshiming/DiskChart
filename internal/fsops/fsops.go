// Package fsops provides host file system operations: reveal in file manager and guarded delete.
package fsops

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// ErrRefused is returned when Delete refuses a protected or malformed path.
var ErrRefused = errors.New("fsops: refused to delete protected or malformed path")

// Guard decides which paths Delete must never remove.
type Guard struct {
	Protected func(path string) bool // gets an absolute, clean, non-volume-root path; true refuses it; nil protects nothing more
}

// Delete permanently removes path unless it is malformed or protected.
func (g Guard) Delete(path string) error {
	switch {
	case !filepath.IsAbs(path):
		return fmt.Errorf("%w: path is not absolute", ErrRefused)
	case path != filepath.Clean(path):
		return fmt.Errorf("%w: path is not clean", ErrRefused)
	case filepath.Clean(path) == filepath.VolumeName(path)+string(filepath.Separator):
		return fmt.Errorf("%w: path is a volume root", ErrRefused)
	case g.Protected != nil && g.Protected(path):
		return fmt.Errorf("%w: path is protected", ErrRefused)
	}
	if _, err := os.Lstat(path); err != nil {
		return err
	}
	return os.RemoveAll(path)
}
