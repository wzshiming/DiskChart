package advice

import "path/filepath"

// common registers the anchors shared by every GOOS.
func (b *builder) common() {
	b.add(point, Volume, b.mounts...)
	b.add(point, Home, b.home)
	videos := "Videos"
	if b.goos == "darwin" {
		videos = "Movies"
	}
	b.add(point, UserData,
		b.h("Desktop"), b.h("Documents"), b.h("Downloads"), b.h("Pictures"), b.h("Music"), b.h(videos),
		b.h(".ssh"), b.h(".gnupg"),
	)
	b.add(content, Temp, b.tmp)
	b.add(content, Cache, b.h(".cache"), b.h(".npm", "_cacache"), b.h(".yarn", "cache"))
	b.add(content, Rebuildable,
		b.gomodcache(),
		sub(b.env("CARGO_HOME", b.h(".cargo")), "registry"),
		sub(b.env("GRADLE_USER_HOME", b.h(".gradle")), "caches"),
	)
	b.name(Rebuildable, "node_modules", "__pycache__", ".pytest_cache", ".mypy_cache", ".ruff_cache")
}

// gomodcache resolves GOMODCACHE, then the first GOPATH entry, then ~/go.
func (b *builder) gomodcache() string {
	if v := abs(b.getenv("GOMODCACHE")); v != "" {
		return v
	}
	if l := filepath.SplitList(b.getenv("GOPATH")); len(l) > 0 {
		if gp := abs(l[0]); gp != "" {
			return filepath.Join(gp, "pkg", "mod")
		}
	}
	return b.h("go", "pkg", "mod")
}

// user registers the user's own rules; they never outrank built-in reasons on the same node.
func (b *builder) user(u UserRules) {
	for _, p := range u.Protected {
		b.add(subtree, Custom, b.expand(p))
	}
	for _, p := range u.Recommended {
		b.add(content, Custom, b.expand(p))
	}
	b.name(Custom, u.Names...)
}
