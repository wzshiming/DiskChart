// Package volumes lists mounted volumes worth scanning and all mount points.
package volumes

import (
	"context"
	"fmt"
	"path"
	"path/filepath"
	"runtime"
	"slices"
	"strings"

	"github.com/shirou/gopsutil/v4/disk"
)

// Volume is a mounted, scannable filesystem with its capacity in bytes.
type Volume struct {
	Mountpoint, Device, FSType, Label string
	Total, Used, Free                 uint64
}

var partitions = func(ctx context.Context, all bool) ([]disk.PartitionStat, error) {
	return disk.PartitionsWithContext(ctx, all)
}

var usage = func(ctx context.Context, path string) (*disk.UsageStat, error) {
	return disk.UsageWithContext(ctx, path)
}

var pseudoFS = map[string]bool{
	"autofs": true, "bpf": true, "binfmt_misc": true, "cgroup": true, "cgroup2": true,
	"configfs": true, "debugfs": true, "devfs": true, "devpts": true, "devtmpfs": true,
	"efivarfs": true, "fuse.gvfsd-fuse": true, "fuse.portal": true, "fusectl": true,
	"hugetlbfs": true, "mqueue": true, "nsfs": true, "overlay": true, "proc": true,
	"pstore": true, "ramfs": true, "rpc_pipefs": true, "securityfs": true, "selinuxfs": true,
	"squashfs": true, "sysfs": true, "tmpfs": true, "tracefs": true,
}

// List returns the scannable volumes with capacity, sorted by mount point.
func List(ctx context.Context) ([]Volume, error) {
	parts, err := partitions(ctx, true)
	if err != nil {
		return nil, err
	}
	vols := []Volume{}
	kept := 0
	var lastErr error
	for _, p := range parts {
		if !keep(runtime.GOOS, p) {
			continue
		}
		kept++
		mp := normalizeMountpoint(p.Mountpoint)
		u, err := usage(ctx, mp)
		if err != nil {
			lastErr = err
			continue
		}
		vols = append(vols, Volume{
			Mountpoint: mp,
			Device:     p.Device,
			FSType:     p.Fstype,
			Label:      label(runtime.GOOS, mp),
			Total:      u.Total,
			Used:       u.Used,
			Free:       u.Free,
		})
	}
	if kept > 0 && len(vols) == 0 {
		return nil, fmt.Errorf("volumes: no usable volumes: %w", lastErr)
	}
	slices.SortFunc(vols, func(a, b Volume) int { return strings.Compare(a.Mountpoint, b.Mountpoint) })
	return vols, nil
}

// Mountpoints returns every mount point, including pseudo filesystems, cleaned, deduplicated and sorted.
func Mountpoints(ctx context.Context) ([]string, error) {
	parts, err := partitions(ctx, true)
	if err != nil {
		return nil, err
	}
	mps := make([]string, 0, len(parts))
	for _, p := range parts {
		mps = append(mps, normalizeMountpoint(p.Mountpoint))
	}
	slices.Sort(mps)
	return slices.Compact(mps), nil
}

// normalizeMountpoint turns a bare Windows drive ("C:") into its root ("C:\"); Clean would yield "C:.".
func normalizeMountpoint(mp string) string {
	if len(mp) == 2 && mp[1] == ':' && ('a' <= mp[0] && mp[0] <= 'z' || 'A' <= mp[0] && mp[0] <= 'Z') {
		return mp + `\`
	}
	return filepath.Clean(mp)
}

func keep(goos string, p disk.PartitionStat) bool {
	if p.Fstype == "" || pseudoFS[strings.ToLower(p.Fstype)] {
		return false
	}
	mp := p.Mountpoint
	switch goos {
	case "darwin":
		if mp == "/" {
			return true
		}
		return !strings.HasPrefix(mp, "/System/Volumes/") &&
			!strings.HasPrefix(mp, "/private/var/vm") &&
			!strings.HasPrefix(mp, "/Volumes/.timemachine") &&
			!strings.HasPrefix(mp, "/Volumes/com.apple.TimeMachine.localsnapshots") &&
			!strings.HasPrefix(mp, "/Library/Developer/CoreSimulator/Volumes/") &&
			mp != "/dev"
	case "linux":
		return !strings.HasPrefix(mp, "/snap/") &&
			!strings.HasPrefix(mp, "/boot/efi") &&
			!strings.HasPrefix(mp, "/var/lib/docker/")
	}
	return true
}

func label(goos, mountpoint string) string {
	if goos == "windows" {
		s := strings.TrimRight(mountpoint, `\/`)
		if s == "" || (len(s) == 2 && s[1] == ':') {
			return ""
		}
		return s[strings.LastIndexAny(s, `\/`)+1:]
	}
	c := path.Clean(mountpoint)
	if c == "/" {
		return ""
	}
	return path.Base(c)
}
