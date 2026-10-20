package advice

import "path/filepath"

// windows registers the Windows anchors; environment defaults assume the system drive is C:.
func (b *builder) windows() {
	systemRoot := b.env("SystemRoot", `C:\Windows`)
	b.add(subtree, System,
		systemRoot,
		b.env("ProgramFiles", `C:\Program Files`),
		b.env("ProgramFiles(x86)", `C:\Program Files (x86)`),
	)
	for _, m := range b.mounts {
		b.add(subtree, System,
			filepath.Join(m, "System Volume Information"),
			filepath.Join(m, "Recovery"),
			filepath.Join(m, "Boot"),
			filepath.Join(m, "EFI"),
		)
		b.add(point, System, filepath.Join(m, "Users"), filepath.Join(m, "$Recycle.Bin"))
		if b.uid != "" {
			b.add(content, Trash, filepath.Join(m, "$Recycle.Bin", b.uid))
		}
	}
	b.add(point, System, b.env("ProgramData", `C:\ProgramData`))
	local := b.env("LOCALAPPDATA", b.h("AppData", "Local"))
	b.add(point, UserData,
		b.h("AppData"),
		b.env("APPDATA", b.h("AppData", "Roaming")),
		local,
		b.h("AppData", "LocalLow"),
		b.env("OneDrive", b.h("OneDrive")),
	)
	b.add(content, Temp, b.tmp, sub(systemRoot, "Temp"))
	b.add(content, Cache,
		sub(local, "Microsoft", "Windows", "INetCache"),
		sub(local, "Microsoft", "Windows", "Explorer"),
		sub(local, "npm-cache"),
		sub(local, "pip", "cache"),
		sub(local, "Yarn", "Cache"),
		sub(local, "go-build"),
	)
}
