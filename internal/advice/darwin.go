package advice

import "path/filepath"

// darwin registers the macOS anchors.
func (b *builder) darwin() {
	b.add(subtree, System, "/System", "/usr", "/bin", "/sbin", "/private", "/dev", "/Library/Apple")
	b.add(point, System, "/Library", "/Volumes", "/Users", "/etc", "/var", "/tmp")
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
