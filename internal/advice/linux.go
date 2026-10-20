package advice

import "path/filepath"

// linux registers the Linux anchors.
func (b *builder) linux() {
	b.add(subtree, System,
		"/usr", "/bin", "/sbin", "/lib", "/lib32", "/lib64", "/libx32", "/etc", "/boot",
		"/proc", "/sys", "/dev", "/run", "/var/lib", "/snap",
	)
	b.add(point, System, "/var", "/var/log", "/home", "/root", "/mnt", "/media", "/opt")
	b.add(point, UserData, b.h(".config"), b.h(".local"), b.h(".local", "share"))
	b.add(content, Cache, b.env("XDG_CACHE_HOME", b.h(".cache")))
	b.add(content, Temp, "/tmp", "/var/tmp")
	b.add(content, Trash, b.h(".local", "share", "Trash"))
	if b.uid != "" {
		for _, m := range b.mounts {
			b.add(content, Trash, filepath.Join(m, ".Trash-"+b.uid), filepath.Join(m, ".Trash", b.uid))
		}
	}
}
