// Package fsops provides host file system operations: reveal in file manager and guarded delete.
package fsops

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
)

// ErrRefused is returned when Delete refuses a protected or malformed path.
var ErrRefused = errors.New("fsops: refused to delete protected or malformed path")

// Guard holds the paths Delete must never remove.
type Guard struct {
	Mountpoints []string // absolute, cleaned mount points refused as delete targets
	Home        string   // user home directory refused as delete target; "" disables
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
	case slices.ContainsFunc(g.Mountpoints, func(m string) bool { return filepath.Clean(m) == path }):
		return fmt.Errorf("%w: path is a mount point", ErrRefused)
	case g.Home != "" && path == filepath.Clean(g.Home):
		return fmt.Errorf("%w: path is the home directory", ErrRefused)
	}
	if _, err := os.Lstat(path); err != nil {
		return err
	}
	return os.RemoveAll(path)
}
