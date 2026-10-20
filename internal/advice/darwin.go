package advice

import (
	"path/filepath"
	"strings"
)

// darwin registers the macOS anchors.
func (b *builder) darwin() {
	b.add(subtree, System, "/System", "/usr", "/bin", "/sbin", "/private", "/dev", "/Library/Apple")
	b.add(subtree, System, "/private/var", "/private/etc") // already under /private; listed so privateAlias covers /var and /etc too
	b.add(point, System, "/Library", "/Volumes", "/Users", "/tmp")
	b.add(point, UserData,
		b.h("Library"),
		b.h("Library", "Application Support"),
		b.h("Library", "Preferences"),
		b.h("Library", "Mail"),
		b.h("Library", "Messages"),
		b.h("Library", "Containers"),
		b.h("Library", "Group Containers"),
		b.h("Library", "Keychains"),
		b.h("Library", "Mobile Documents"),
	)
	b.add(content, Cache,
		b.h("Library", "Caches"),
		"/Library/Caches",
		b.h("Library", "Developer", "CoreSimulator", "Caches"),
	)
	if b.tmp != "" && filepath.Base(b.tmp) == "T" { // per-user confstr dirs: .../T is temp, its sibling C is cache
		b.add(content, Cache, filepath.Join(filepath.Dir(b.tmp), "C"))
	}
	b.add(content, Rebuildable,
		b.h("Library", "Developer", "Xcode", "DerivedData"),
		b.h("Library", "Developer", "Xcode", "iOS DeviceSupport"),
		b.h("Library", "Developer", "Xcode", "watchOS DeviceSupport"),
		b.h("Library", "Developer", "Xcode", "tvOS DeviceSupport"),
		b.h("Library", "Developer", "Xcode", "visionOS DeviceSupport"),
	)
	b.add(content, Temp, "/private/tmp", "/tmp", "/cores")
	b.add(content, Trash, b.h(".Trash"))
	b.add(content, Logs, b.h("Library", "Logs"))
	if b.uid != "" {
		for _, m := range b.mounts {
			b.add(content, Trash, filepath.Join(m, ".Trashes", b.uid))
		}
	}
}

// privateAlias returns the other spelling of a path at or beneath /var, /tmp or /etc, which macOS
// symlinks into /private, so scans rooted on either side hit the same anchors; "" for other paths.
func privateAlias(p string) string {
	const private = "/private"
	for _, d := range [...]string{"/var", "/tmp", "/etc"} {
		switch {
		case within(p, private+d):
			return p[len(private):]
		case within(p, d):
			return private + p
		}
	}
	return ""
}

// within reports whether p equals dir or lies beneath it.
func within(p, dir string) bool {
	return strings.HasPrefix(p, dir) && (len(p) == len(dir) || p[len(dir)] == '/')
}
