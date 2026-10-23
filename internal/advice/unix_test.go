//go:build unix

package advice_test

import (
	"testing"

	"github.com/wzshiming/DiskChart/internal/advice"
)

func darwinConfig() advice.Config {
	return advice.Config{
		GOOS:        "darwin",
		Home:        "/Users/u",
		TempDir:     "/private/var/folders/ab/cd/T",
		UID:         "501",
		Mountpoints: []string{"/", "/dev", "/System/Volumes/Data", "/Volumes/USB"},
		Getenv:      env(nil),
	}
}

func TestDarwinRules(t *testing.T) {
	runCases(t, advice.New(darwinConfig()), []classifyCase{
		{path: "/", want: protected(advice.Volume)},
		{path: "/dev", want: protected(advice.Volume)},
		{path: "/System/Volumes/Data", want: protected(advice.Volume)},
		{path: "/Volumes/USB", want: protected(advice.Volume)},
		{path: "/System", want: protected(advice.System)},
		{path: "/usr/lib/node_modules", dir: true, want: protected(advice.System)},
		{path: "/private/tmp", want: protected(advice.System)},
		{path: "/private/var/folders/ab/cd", want: protected(advice.System)},
		{path: "/tmp", want: protected(advice.System)},
		{path: "/Library", want: protected(advice.System)},
		{path: "/Library/Apple/x", want: protected(advice.System)},
		{path: "/Users", want: protected(advice.System)},
		{path: "/Volumes", want: protected(advice.System)},
		{path: "/etc", want: protected(advice.System)},
		{path: "/dev/null", want: protected(advice.System)},
		{path: "/System/Volumes", want: protected(advice.Contains)},
		{path: "/Users/u", want: protected(advice.Home)},
		{path: "/Users/u/Library", want: protected(advice.UserData)},
		{path: "/Users/u/Library/Application Support", want: protected(advice.UserData)},
		{path: "/Users/u/Movies", want: protected(advice.UserData)},
		{path: "/Library/Caches", want: none},
		{path: "/Library/Caches/x", want: recommended(advice.Cache)},
		{path: "/Users/u/Library/Caches/x", want: recommended(advice.Cache)},
		{path: "/private/var/folders/ab/cd/C/x", want: recommended(advice.Cache)},
		{path: "/private/tmp/x", want: recommended(advice.Temp)},
		{path: "/tmp/x", want: recommended(advice.Temp)},
		{path: "/private/var/folders/ab/cd/T/x", want: recommended(advice.Temp)},
		{path: "/cores/x", want: recommended(advice.Temp)},
		{path: "/Users/u/.Trash/x", want: recommended(advice.Trash)},
		{path: "/Volumes/USB/.Trashes/501/x", want: recommended(advice.Trash)},
		{path: "/Volumes/USB/.Trashes/501", want: none},
		{path: "/Volumes/USB/.Trashes/502/x", want: none},
		{path: "/Volumes/USB/.Trashes", want: none},
		{path: "/Users/u/Library/Logs/x", want: recommended(advice.Logs)},
		{path: "/Users/u/Library/Developer/Xcode/DerivedData/x", want: recommended(advice.Rebuildable)},
		{path: "/Users/u/Library/Developer/Xcode/iOS DeviceSupport/x", want: recommended(advice.Rebuildable)},
		{path: "/USERS/U", want: protected(advice.Home)},
	})
}

func TestDarwinEmptyUIDOmitsVolumeTrash(t *testing.T) {
	c := darwinConfig()
	c.UID = ""
	runCases(t, advice.New(c), []classifyCase{
		{path: "/Volumes/USB/.Trashes/501/x", want: none},
		{path: "/Users/u/.Trash/x", want: recommended(advice.Trash)},
	})
}

func linuxConfig() advice.Config {
	return advice.Config{
		GOOS:        "linux",
		Home:        "/home/u",
		TempDir:     "/tmp",
		UID:         "1000",
		Mountpoints: []string{"/", "/proc", "/dev", "/run", "/boot/efi", "/home", "/media/u/usb"},
		Getenv:      env(nil),
	}
}

func TestLinuxRules(t *testing.T) {
	runCases(t, advice.New(linuxConfig()), []classifyCase{
		{path: "/", want: protected(advice.Volume)},
		{path: "/home", want: protected(advice.Volume)},
		{path: "/boot/efi", want: protected(advice.Volume)},
		{path: "/home/u", want: protected(advice.Home)},
		{path: "/boot", want: protected(advice.System)},
		{path: "/var", want: protected(advice.System)},
		{path: "/var/log", want: protected(advice.System)},
		{path: "/opt", want: protected(advice.System)},
		{path: "/root", want: protected(advice.System)},
		{path: "/media", want: protected(advice.System)},
		{path: "/var/lib/x", want: protected(advice.System)},
		{path: "/snap/core/1", want: protected(advice.System)},
		{path: "/run/user/1", want: protected(advice.System)},
		{path: "/usr/lib/node_modules", dir: true, want: protected(advice.System)},
		{path: "/media/u", want: protected(advice.Contains)},
		{path: "/var/log/x", want: none},
		{path: "/opt/x", want: none},
		{path: "/var/cache/x", want: none},
		{path: "/tmp", want: none},
		{path: "/tmp/x", want: recommended(advice.Temp)},
		{path: "/var/tmp/x", want: recommended(advice.Temp)},
		{path: "/home/u/.config", want: protected(advice.UserData)},
		{path: "/home/u/.config/x", want: none},
		{path: "/home/u/.local/share/Trash", want: none},
		{path: "/home/u/.local/share/Trash/files/x", want: recommended(advice.Trash)},
		{path: "/media/u/usb/.Trash-1000/files/x", want: recommended(advice.Trash)},
		{path: "/media/u/usb/.Trash/1000/files/x", want: recommended(advice.Trash)},
		{path: "/media/u/usb/.Trash-1001/x", want: none},
		{path: "/home/u/.cache/x", want: recommended(advice.Cache)},
		{path: "/HOME/U", want: none},
	})
}

func TestLinuxXDGCacheHome(t *testing.T) {
	c := linuxConfig()
	c.Getenv = env(map[string]string{"XDG_CACHE_HOME": "/home/u/xc"})
	runCases(t, advice.New(c), []classifyCase{
		{path: "/home/u/xc", want: none},
		{path: "/home/u/xc/x", want: recommended(advice.Cache)},
	})
}

// Without Mountpoints, the volume root of Home still gets the per-volume anchors.
func TestHomeVolumeRootImplied(t *testing.T) {
	t.Run("darwin", func(t *testing.T) {
		r := advice.New(advice.Config{
			GOOS:        "darwin",
			Home:        "/Users/u",
			UID:         "501",
			Mountpoints: nil,
			Getenv:      env(nil),
		})
		runCases(t, r, []classifyCase{
			{path: "/.Trashes/501/x", want: recommended(advice.Trash)},
			{path: "/Volumes/USB/.Trashes/501/x", want: none},
			{path: "/", want: protected(advice.Volume)},
		})
	})
	t.Run("linux", func(t *testing.T) {
		r := advice.New(advice.Config{
			GOOS:        "linux",
			Home:        "/home/u",
			UID:         "1000",
			Mountpoints: nil,
			Getenv:      env(nil),
		})
		runCases(t, r, []classifyCase{
			{path: "/.Trash-1000/files/x", want: recommended(advice.Trash)},
			{path: "/.Trash/1000/x", want: recommended(advice.Trash)},
		})
	})
}

// On darwin /var, /tmp and /etc are symlinks into /private, so every anchor under either form must also match the other.
func TestDarwinPrivateAliases(t *testing.T) {
	base := advice.Config{
		GOOS:        "darwin",
		Home:        "/Users/u",
		UID:         "501",
		Mountpoints: []string{"/"},
		Getenv:      env(nil),
	}
	t.Run("TempDir in /var", func(t *testing.T) {
		c := base
		c.TempDir = "/var/folders/ab/cd/T"
		runCases(t, advice.New(c), []classifyCase{
			{path: "/var/folders/ab/cd/T/x", want: recommended(advice.Temp)},
			{path: "/private/var/folders/ab/cd/T/x", want: recommended(advice.Temp)},
			{path: "/var/folders/ab/cd/C/x", want: recommended(advice.Cache)},
			{path: "/private/var/folders/ab/cd/C/x", want: recommended(advice.Cache)},
			{path: "/var/folders/ab/cd/T", want: protected(advice.System)},
			{path: "/private/var/folders/ab/cd/T", want: protected(advice.System)},
		})
	})
	t.Run("TempDir in /private/var", func(t *testing.T) {
		c := base
		c.TempDir = "/private/var/folders/ab/cd/T"
		runCases(t, advice.New(c), []classifyCase{
			{path: "/var/folders/ab/cd/T/x", want: recommended(advice.Temp)},
		})
	})
	t.Run("user rules", func(t *testing.T) {
		c := base
		c.User = advice.UserRules{
			Protected:   []string{"/private/etc/keep"},
			Recommended: []string{"/var/data/cache"},
		}
		runCases(t, advice.New(c), []classifyCase{
			{path: "/private/var/data/cache/x", want: recommended(advice.Custom)},
			{path: "/var/data/cache/x", want: recommended(advice.Custom)},
			{path: "/etc/keep", want: protected(advice.Custom)},
			{path: "/etc/keep/x", want: protected(advice.Custom)},
			{path: "/etc", want: protected(advice.System)},
			{path: "/private/etc/keep/x", want: protected(advice.Custom)},
		})
	})
	t.Run("linux has no aliases", func(t *testing.T) {
		c := base
		c.GOOS = "linux"
		c.Home = "/home/u"
		c.UID = "1000"
		c.TempDir = "/var/folders/ab/cd/T"
		runCases(t, advice.New(c), []classifyCase{
			{path: "/private/var/folders/ab/cd/T/x", want: none},
		})
	})
}

// On darwin /private/var and /private/etc are System subtrees, so under either spelling everything a deeper
// anchor does not cover is protected, while deeper content and volume anchors still win.
func TestDarwinPrivateSubtrees(t *testing.T) {
	c := advice.Config{
		GOOS:        "darwin",
		Home:        "/Users/u",
		TempDir:     "/private/var/folders/ab/cd/T",
		UID:         "501",
		Mountpoints: []string{"/", "/private/var/vm"},
		Getenv:      env(nil),
	}
	t.Run("darwin", func(t *testing.T) {
		runCases(t, advice.New(c), []classifyCase{
			{path: "/var/db/x", want: protected(advice.System)},
			{path: "/etc/hosts", want: protected(advice.System)},
			{path: "/var/log/x", want: protected(advice.System)},
			{path: "/var/folders/ab/cd", want: protected(advice.System)},
			{path: "/var/folders/ab/cd/T/x", want: recommended(advice.Temp)},
			{path: "/private/var/folders/ab/cd/T/x", want: recommended(advice.Temp)},
			{path: "/var/folders/ab/cd/C/x", want: recommended(advice.Cache)},
			{path: "/var/vm", want: protected(advice.Volume)},
			{path: "/private/var/vm", want: protected(advice.Volume)},
			{path: "/var", want: protected(advice.System)},
			{path: "/etc", want: protected(advice.System)},
			{path: "/private", want: protected(advice.System)},
			{path: "/private/var", want: protected(advice.System)},
			{path: "/private/etc", want: protected(advice.System)},
			{path: "/tmp", want: protected(advice.System)},
			{path: "/tmp/x", want: recommended(advice.Temp)},
			{path: "/Users/u/x", want: none},
		})
	})
	t.Run("linux has no /private subtrees", func(t *testing.T) {
		cc := c
		cc.GOOS = "linux"
		runCases(t, advice.New(cc), []classifyCase{
			{path: "/var/db/x", want: none},
			{path: "/var/lib/x", want: protected(advice.System)},
			{path: "/private/var/db/x", want: none},
			{path: "/var/vm", want: none},
			{path: "/private/var/vm", want: protected(advice.Volume)},
		})
	})
}

// privateAlias mirrors only /var, /tmp and /etc themselves and their contents: a sibling whose name merely
// shares the prefix gets no alias in either direction.
func TestDarwinAliasBoundary(t *testing.T) {
	c := darwinConfig()
	c.User = advice.UserRules{
		Protected:   []string{"/variable/x", "/private/etcd/keep"},
		Recommended: []string{"/tmpfs/data", "/private/vartmp/data"},
	}
	runCases(t, advice.New(c), []classifyCase{
		{path: "/tmpfs/data/x", want: recommended(advice.Custom)},
		{path: "/private/tmpfs/data/x", want: protected(advice.System)}, // the /private subtree, not the user rule
		{path: "/variable/x", want: protected(advice.Custom)},
		{path: "/private/variable/x", want: protected(advice.System)},
		{path: "/private/vartmp/data/x", want: recommended(advice.Custom)},
		{path: "/vartmp/data/x", want: none},
		{path: "/private/etcd/keep/x", want: protected(advice.Custom)},
		{path: "/etcd/keep/x", want: none},
	})
}
