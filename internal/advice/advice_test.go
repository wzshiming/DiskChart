package advice_test

import (
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/wzshiming/DiskChart/internal/advice"
)

const sep = string(filepath.Separator)

var none = advice.Verdict{}

func protected(r advice.Reason) advice.Verdict {
	return advice.Verdict{Class: advice.Protected, Reason: r}
}

func recommended(r advice.Reason) advice.Verdict {
	return advice.Verdict{Class: advice.Recommended, Reason: r}
}

// env returns a Getenv that only knows the given variables.
func env(vars map[string]string) func(string) string {
	return func(key string) string { return vars[key] }
}

// testConfig builds the host-agnostic fixture: GOOS "test" enables only the
// common table, so nothing outside home, tmp and the mountpoints is anchored.
func testConfig(t testing.TB) (c advice.Config, home, tmp string) {
	t.Helper()
	root := t.TempDir()
	home = filepath.Join(root, "home")
	tmp = filepath.Join(root, "tmp")
	c = advice.Config{
		GOOS:        "test",
		Home:        home,
		TempDir:     tmp,
		UID:         "501",
		Mountpoints: []string{filepath.VolumeName(home) + sep, filepath.Join(home, ".cache", "mnt")},
		Getenv:      env(nil),
	}
	return c, home, tmp
}

type classifyCase struct {
	path string
	dir  bool
	want advice.Verdict
}

func (c classifyCase) name() string {
	name := c.path
	if name == "" {
		name = "<empty>"
	}
	if c.dir {
		name += " (dir)"
	}
	return name
}

// classify calls Classify and checks the invariants every verdict must hold.
func classify(t *testing.T, r *advice.Rules, path string, isDir bool) advice.Verdict {
	t.Helper()
	got := r.Classify(path, isDir)
	if got.Reason == advice.Root {
		t.Errorf("Classify(%q, %t) = %+v; Root is reserved for the scan root", path, isDir, got)
	}
	if (got.Class == advice.None) != (got.Reason == "") {
		t.Errorf("Classify(%q, %t) = %+v; want Class None exactly when Reason is empty", path, isDir, got)
	}
	return got
}

func runCases(t *testing.T, r *advice.Rules, cases []classifyCase) {
	t.Helper()
	for _, c := range cases {
		t.Run(c.name(), func(t *testing.T) {
			if got := classify(t, r, c.path, c.dir); got != c.want {
				t.Errorf("Classify(%q, %t) = %+v, want %+v", c.path, c.dir, got, c.want)
			}
		})
	}
}

func TestNonAbsolutePathIsOrdinary(t *testing.T) {
	c, _, _ := testConfig(t)
	runCases(t, advice.New(c), []classifyCase{
		{path: "", want: none},
		{path: ".", want: none},
		{path: filepath.Join("rel", "x"), want: none},
		{path: filepath.Join("rel", "node_modules"), dir: true, want: none},
	})
}

func TestVolumeRootAndHomeAreProtected(t *testing.T) {
	c, home, _ := testConfig(t)
	cases := []classifyCase{
		{path: filepath.VolumeName(home) + sep, want: protected(advice.Volume)},
		{path: home, want: protected(advice.Home)},
		{path: home + sep, want: protected(advice.Home)},
		{path: home + sep + "." + sep + "x" + sep + "..", want: protected(advice.Home)},
	}
	t.Run("with mountpoints", func(t *testing.T) {
		runCases(t, advice.New(c), cases)
	})
	t.Run("without mountpoints", func(t *testing.T) {
		c.Mountpoints = nil
		runCases(t, advice.New(c), cases)
	})
}

func TestHomeOnMountpointIsVolume(t *testing.T) {
	c, home, _ := testConfig(t)
	c.Mountpoints = []string{home}
	runCases(t, advice.New(c), []classifyCase{
		{path: home, want: protected(advice.Volume)},
		{path: filepath.Join(home, "Desktop"), want: protected(advice.UserData)},
		{path: filepath.Join(home, ".cache", "o"), want: recommended(advice.Cache)},
	})
}

func TestPointAnchorProtectsOnlyItself(t *testing.T) {
	c, home, _ := testConfig(t)
	var cases []classifyCase
	for _, name := range []string{"Desktop", "Documents", "Downloads", "Pictures", "Music", "Videos", ".ssh", ".gnupg"} {
		cases = append(cases,
			classifyCase{path: filepath.Join(home, name), want: protected(advice.UserData)},
			classifyCase{path: filepath.Join(home, name, "a"), want: none},
		)
	}
	cases = append(cases, classifyCase{path: filepath.Join(home, ".ssh", "id"), want: none})
	runCases(t, advice.New(c), cases)
}

func TestAncestorOfProtectedAnchorIsContains(t *testing.T) {
	c, home, tmp := testConfig(t)
	runCases(t, advice.New(c), []classifyCase{
		{path: filepath.Join(home, ".cache"), want: protected(advice.Contains)},
		{path: filepath.Join(home, ".cache"), dir: true, want: protected(advice.Contains)},
		{path: filepath.Join(home, ".cache", "mnt"), want: protected(advice.Volume)},
		{path: filepath.Join(home, ".cache", "o"), want: recommended(advice.Cache)},
		{path: filepath.Dir(tmp), want: protected(advice.Contains)},
	})
}

func TestContentsAnchorRecommendsOnlyItsContents(t *testing.T) {
	c, home, tmp := testConfig(t)
	runCases(t, advice.New(c), []classifyCase{
		{path: tmp, want: none},
		{path: tmp, dir: true, want: none},
		{path: filepath.Join(tmp, "x"), want: recommended(advice.Temp)},
		{path: filepath.Join(tmp, "x", "y"), dir: true, want: recommended(advice.Temp)},
		{path: filepath.Join(home, ".npm", "_cacache"), want: none},
		{path: filepath.Join(home, ".npm", "_cacache", "x"), want: recommended(advice.Cache)},
		{path: filepath.Join(home, ".yarn", "cache", "x"), want: recommended(advice.Cache)},
	})
}

func TestVolumeRootContentsAnchorDropped(t *testing.T) {
	c, home, tmp := testConfig(t)
	vr := filepath.VolumeName(home) + sep
	t.Run("TempDir", func(t *testing.T) {
		cc := c
		cc.TempDir = vr
		runCases(t, advice.New(cc), []classifyCase{
			{path: filepath.Join(home, "x"), want: none},
			{path: vr + "x", want: none},
			{path: vr, want: protected(advice.Volume)},
		})
	})
	t.Run("GOMODCACHE", func(t *testing.T) {
		cc := c
		cc.Getenv = env(map[string]string{"GOMODCACHE": vr})
		runCases(t, advice.New(cc), []classifyCase{
			{path: filepath.Join(home, "pkg", "mod", "x"), want: none},
		})
	})
	t.Run("User.Recommended", func(t *testing.T) {
		cc := c
		cc.User.Recommended = []string{vr}
		runCases(t, advice.New(cc), []classifyCase{
			{path: filepath.Join(home, "x"), want: none},
		})
	})
	t.Run("ordinary TempDir is kept", func(t *testing.T) {
		runCases(t, advice.New(c), []classifyCase{
			{path: filepath.Join(tmp, "x"), want: recommended(advice.Temp)},
		})
	})
}

func TestContentsAnchorBeatsNameRule(t *testing.T) {
	c, home, _ := testConfig(t)
	runCases(t, advice.New(c), []classifyCase{
		{path: filepath.Join(home, ".cache", "p", "node_modules"), dir: true, want: recommended(advice.Cache)},
		{path: filepath.Join(home, ".cache", "p", "node_modules", "a.js"), want: recommended(advice.Cache)},
	})
}

func TestNameRule(t *testing.T) {
	c, home, _ := testConfig(t)
	var cases []classifyCase
	for _, name := range []string{"node_modules", "__pycache__", ".pytest_cache", ".mypy_cache", ".ruff_cache"} {
		p := filepath.Join(home, "p", name)
		cases = append(cases,
			classifyCase{path: p, dir: true, want: recommended(advice.Rebuildable)},
			classifyCase{path: p, want: none},
			classifyCase{path: filepath.Join(p, "a.js"), want: recommended(advice.Rebuildable)},
		)
	}
	cases = append(cases,
		classifyCase{path: filepath.Join(home, "Documents", "p", "node_modules"), dir: true, want: recommended(advice.Rebuildable)},
		classifyCase{path: filepath.Join(home, "p", "node_modules", "sub", "b.js"), want: recommended(advice.Rebuildable)},
		classifyCase{path: filepath.Join(home, "p", "x"), dir: true, want: none},
	)
	runCases(t, advice.New(c), cases)
}

func TestEnvAnchors(t *testing.T) {
	c, home, _ := testConfig(t)
	root := filepath.Dir(home)
	gomod := filepath.Join(root, "gomod")
	gopathA := filepath.Join(root, "gpa")
	gopathB := filepath.Join(root, "gpb")
	cargo := filepath.Join(root, "cargo")

	t.Run("absolute GOMODCACHE", func(t *testing.T) {
		cc := c
		cc.Getenv = env(map[string]string{"GOMODCACHE": gomod})
		runCases(t, advice.New(cc), []classifyCase{
			{path: filepath.Join(gomod, "x"), want: recommended(advice.Rebuildable)},
			{path: filepath.Join(gomod, "cache", "download"), dir: true, want: recommended(advice.Rebuildable)},
			{path: gomod, want: none},
		})
	})
	t.Run("first GOPATH element", func(t *testing.T) {
		cc := c
		cc.Getenv = env(map[string]string{"GOPATH": gopathA + string(filepath.ListSeparator) + gopathB})
		runCases(t, advice.New(cc), []classifyCase{
			{path: filepath.Join(gopathA, "pkg", "mod", "x"), want: recommended(advice.Rebuildable)},
			{path: filepath.Join(gopathB, "pkg", "mod", "x"), want: none},
		})
	})
	t.Run("defaults under home", func(t *testing.T) {
		runCases(t, advice.New(c), []classifyCase{
			{path: filepath.Join(home, "go", "pkg", "mod", "x"), want: recommended(advice.Rebuildable)},
			{path: filepath.Join(home, ".cargo", "registry", "x"), want: recommended(advice.Rebuildable)},
			{path: filepath.Join(home, ".gradle", "caches", "x"), want: recommended(advice.Rebuildable)},
			{path: filepath.Join(home, ".cargo", "bin", "x"), want: none},
		})
	})
	t.Run("relative values fall back to defaults", func(t *testing.T) {
		cc := c
		cc.Getenv = env(map[string]string{
			"CARGO_HOME":       filepath.Join("rel", "cargo"),
			"GRADLE_USER_HOME": filepath.Join("rel", "gradle"),
		})
		runCases(t, advice.New(cc), []classifyCase{
			{path: filepath.Join(home, ".cargo", "registry", "x"), want: recommended(advice.Rebuildable)},
			{path: filepath.Join(home, ".gradle", "caches", "x"), want: recommended(advice.Rebuildable)},
		})
	})
	t.Run("absolute CARGO_HOME", func(t *testing.T) {
		cc := c
		cc.Getenv = env(map[string]string{"CARGO_HOME": cargo})
		runCases(t, advice.New(cc), []classifyCase{
			{path: filepath.Join(cargo, "registry", "x"), want: recommended(advice.Rebuildable)},
			{path: filepath.Join(cargo, "registry"), want: none},
		})
	})
	t.Run("nil Getenv reads the process environment", func(t *testing.T) {
		t.Setenv("GOMODCACHE", gomod)
		cc := c
		cc.Getenv = nil
		runCases(t, advice.New(cc), []classifyCase{
			{path: filepath.Join(gomod, "x"), want: recommended(advice.Rebuildable)},
		})
	})
}

func TestEmptyHomeOmitsHomeAnchors(t *testing.T) {
	c, home, _ := testConfig(t)
	cargo := filepath.Join(filepath.Dir(home), "cargo")
	c.Home = ""
	c.Mountpoints = nil
	c.Getenv = env(map[string]string{"CARGO_HOME": cargo})
	runCases(t, advice.New(c), []classifyCase{
		{path: home, want: none},
		{path: filepath.Join(home, ".cache", "x"), want: none},
		{path: filepath.Join(home, "go", "pkg", "mod", "x"), want: none},
		{path: filepath.Join(cargo, "registry", "x"), want: recommended(advice.Rebuildable)},
	})
}

func TestBadInputs(t *testing.T) {
	c, home, tmp := testConfig(t)
	t.Run("empty TempDir", func(t *testing.T) {
		cc := c
		cc.TempDir = ""
		runCases(t, advice.New(cc), []classifyCase{{path: filepath.Join(tmp, "x"), want: none}})
	})
	t.Run("relative TempDir", func(t *testing.T) {
		cc := c
		cc.TempDir = filepath.Join("rel", "tmp")
		runCases(t, advice.New(cc), []classifyCase{{path: filepath.Join(tmp, "x"), want: none}})
	})
	t.Run("empty and relative mountpoints", func(t *testing.T) {
		cc := c
		cc.Mountpoints = []string{"", "rel", filepath.Join("rel", "mnt")}
		runCases(t, advice.New(cc), []classifyCase{{path: home, want: protected(advice.Home)}})
	})
	t.Run("nil Getenv", func(t *testing.T) {
		cc := c
		cc.Getenv = nil
		runCases(t, advice.New(cc), []classifyCase{{path: home, want: protected(advice.Home)}})
	})
	t.Run("invalid user rules", func(t *testing.T) {
		cc := c
		cc.User = advice.UserRules{
			Protected:   []string{"", "rel", filepath.Join("rel", "keep")},
			Recommended: []string{"", "rel"},
			Names:       []string{"", ".", "..", "a/b", `a\b`},
		}
		runCases(t, advice.New(cc), []classifyCase{
			{path: home, want: protected(advice.Home)},
			{path: filepath.Join(home, "p", "a"), dir: true, want: none},
			{path: filepath.Join(home, "p", "b"), dir: true, want: none},
			{path: filepath.Join(home, "rel", "keep"), want: none},
		})
	})
}

func TestNewDoesNotRetainConfigSlices(t *testing.T) {
	c, home, _ := testConfig(t)
	c.User.Protected = []string{filepath.Join(home, "keep")}
	r := advice.New(c)
	c.Mountpoints[1] = ""
	c.User.Protected[0] = ""
	runCases(t, r, []classifyCase{
		{path: filepath.Join(home, ".cache", "mnt"), want: protected(advice.Volume)},
		{path: filepath.Join(home, "keep", "x"), want: protected(advice.Custom)},
	})
}

func TestCaseFolding(t *testing.T) {
	// A synthetic home at the volume root keeps the OS tables away from the host's temp dir.
	home := filepath.Join(filepath.VolumeName(t.TempDir())+sep, "dc-home")
	nodeModules := filepath.Join(home, "p", "NODE_MODULES")
	folding := "darwin"
	if runtime.GOOS == "windows" {
		folding = "windows"
	}
	t.Run(folding, func(t *testing.T) {
		r := advice.New(advice.Config{GOOS: folding, Home: home, Getenv: env(nil)})
		runCases(t, r, []classifyCase{
			{path: strings.ToUpper(home), want: protected(advice.Home)},
			{path: nodeModules, dir: true, want: recommended(advice.Rebuildable)},
		})
	})
	t.Run("linux", func(t *testing.T) {
		r := advice.New(advice.Config{GOOS: "linux", Home: home, Getenv: env(nil)})
		runCases(t, r, []classifyCase{
			{path: strings.ToUpper(home), want: none},
			{path: nodeModules, dir: true, want: none},
		})
	})
}

func TestAncestorsOfProtectedAnchorsAreProtected(t *testing.T) {
	c, home, _ := testConfig(t)
	r := advice.New(c)
	for _, start := range []string{home, filepath.Join(home, "Desktop"), filepath.Join(home, ".cache", "mnt")} {
		for p := start; ; p = filepath.Dir(p) {
			if got := classify(t, r, p, true); got.Class != advice.Protected {
				t.Errorf("Classify(%q) = %+v, want Protected on the way up from %q", p, got, start)
			}
			if filepath.Dir(p) == p {
				break
			}
		}
	}
}

func TestClassifyIsSafeForConcurrentUse(t *testing.T) {
	c, home, tmp := testConfig(t)
	r := advice.New(c)
	cases := []classifyCase{
		{path: home, want: protected(advice.Home)},
		{path: filepath.Join(home, "Desktop"), want: protected(advice.UserData)},
		{path: filepath.Join(home, ".cache"), want: protected(advice.Contains)},
		{path: filepath.Join(home, ".cache", "o"), want: recommended(advice.Cache)},
		{path: filepath.Join(tmp, "x"), want: recommended(advice.Temp)},
		{path: filepath.Join(home, "p", "node_modules"), dir: true, want: recommended(advice.Rebuildable)},
		{path: filepath.Join(home, "p", "x"), want: none},
	}
	var wg sync.WaitGroup
	for g := 0; g < 64; g++ {
		wg.Go(func() {
			for i := 0; i < 200; i++ {
				for _, c := range cases {
					if got := r.Classify(c.path, c.dir); got != c.want {
						t.Errorf("Classify(%q, %t) = %+v, want %+v", c.path, c.dir, got, c.want)
						return
					}
				}
			}
		})
	}
	wg.Wait()
}

func TestUserRules(t *testing.T) {
	c, home, _ := testConfig(t)
	c.User = advice.UserRules{
		Protected:   []string{filepath.Join("~", "keep"), filepath.Join(home, ".cache", "keep2")},
		Recommended: []string{filepath.Join("~", "junk"), filepath.Join("rel", "ignored")},
		Names:       []string{"target", "bad/name", "", "."},
	}
	runCases(t, advice.New(c), []classifyCase{
		{path: filepath.Join(home, "keep"), want: protected(advice.Custom)},
		{path: filepath.Join(home, "keep", "a", "b"), want: protected(advice.Custom)},
		{path: home, want: protected(advice.Home)},
		{path: filepath.Join(home, ".cache"), want: protected(advice.Contains)},
		{path: filepath.Join(home, ".cache", "keep2"), want: protected(advice.Custom)},
		{path: filepath.Join(home, ".cache", "keep2", "x"), want: protected(advice.Custom)},
		{path: filepath.Join(home, ".cache", "other"), want: recommended(advice.Cache)},
		{path: filepath.Join(home, "junk"), want: none},
		{path: filepath.Join(home, "junk", "x"), want: recommended(advice.Custom)},
		{path: filepath.Join(home, "p", "target"), dir: true, want: recommended(advice.Custom)},
		{path: filepath.Join(home, "p", "target"), want: none},
		{path: filepath.Join(home, "p", "target", "x"), want: recommended(advice.Custom)},
		{path: filepath.Join(home, "p", "bad"), dir: true, want: none},
		{path: filepath.Join(home, "p", "name"), dir: true, want: none},
	})
}

func BenchmarkClassify(b *testing.B) {
	c, home, _ := testConfig(b)
	r := advice.New(c)
	path := filepath.Join(filepath.VolumeName(home)+sep, "s1", "s2", "s3", "s4", "s5", "s6", "s7", "s8", "s9", "s10")
	b.ReportAllocs()
	for b.Loop() {
		r.Classify(path, true)
	}
}
