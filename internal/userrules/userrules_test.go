package userrules_test

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/wzshiming/DiskChart/internal/advice"
	"github.com/wzshiming/DiskChart/internal/userrules"
)

const sep = string(filepath.Separator)

var zero advice.UserRules

// store returns a Store over rules.json in a fresh temp dir that holds nothing else.
func store(t *testing.T) userrules.Store {
	t.Helper()
	return userrules.Store{File: filepath.Join(t.TempDir(), "rules.json")}
}

func readFile(t *testing.T, p string) string {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("ReadFile(%s): %v", p, err)
	}
	return string(b)
}

// dirNames lists the entry names of dir in directory order.
func dirNames(t *testing.T, dir string) []string {
	t.Helper()
	es, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir(%s): %v", dir, err)
	}
	names := make([]string, len(es))
	for i, e := range es {
		names[i] = e.Name()
	}
	return names
}

// oneRule returns UserRules holding only v, in the list named list.
func oneRule(list, v string) advice.UserRules {
	switch list {
	case "Protected":
		return advice.UserRules{Protected: []string{v}}
	case "Recommended":
		return advice.UserRules{Recommended: []string{v}}
	}
	return advice.UserRules{Names: []string{v}}
}

func TestDefaultPath(t *testing.T) {
	t.Run("under UserConfigDir", func(t *testing.T) {
		cfg, err := os.UserConfigDir()
		if err != nil {
			t.Skipf("os.UserConfigDir: %v", err)
		}
		got, err := userrules.DefaultPath()
		if want := filepath.Join(cfg, "DiskChart", "rules.json"); err != nil || got != want {
			t.Fatalf("DefaultPath() = %q, %v, want %q, nil", got, err, want)
		}
	})
	t.Run("without UserConfigDir", func(t *testing.T) {
		for _, k := range []string{"HOME", "XDG_CONFIG_HOME", "AppData"} {
			t.Setenv(k, "")
		}
		if _, err := os.UserConfigDir(); err == nil {
			t.Skipf("clearing HOME, XDG_CONFIG_HOME and AppData does not make os.UserConfigDir fail on %s", runtime.GOOS)
		}
		got, err := userrules.DefaultPath()
		if got != "" || err == nil {
			t.Fatalf("DefaultPath() without a config dir = %q, %v, want \"\" and an error", got, err)
		}
	})
}

func TestPathReturnsFile(t *testing.T) {
	s := store(t)
	if got := s.Path(); got != s.File {
		t.Fatalf("Store{File: %q}.Path() = %q, want File", s.File, got)
	}
}

func TestLoadMissing(t *testing.T) {
	s := userrules.Store{File: filepath.Join(t.TempDir(), "absent", "rules.json")}
	got, err := s.Load()
	if err != nil || !reflect.DeepEqual(got, zero) {
		t.Fatalf("Load() of a missing file = %+v, %v, want zero UserRules and nil", got, err)
	}
	if _, err := os.Stat(filepath.Dir(s.File)); !os.IsNotExist(err) {
		t.Fatalf("Load() left %s with Stat error %v, want it still absent", filepath.Dir(s.File), err)
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "cfg", "DiskChart")
	s := userrules.Store{File: filepath.Join(dir, "rules.json")}
	// Unvalidated and unnormalized items must survive as they are: Save and Load neither check nor clean.
	want := advice.UserRules{
		Protected:   []string{filepath.Join(dir, "keep"), "~/keep", "~"},
		Recommended: []string{"~/junk", "rel/kept", " padded "},
		Names:       []string{"target", "é", "x&y<z>", "."},
	}
	if err := s.Save(want); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := s.Load()
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("Load() after Save = %+v, %v, want %+v, nil", got, err, want)
	}
	if runtime.GOOS == "windows" {
		return
	}
	if fi, err := os.Stat(dir); err != nil || fi.Mode().Perm() != 0o700 {
		t.Errorf("Stat(%s) = %v, %v, want a directory created with mode 0700", dir, fi.Mode(), err)
	}
	if fi, err := os.Stat(s.File); err != nil || fi.Mode().Perm() != 0o600 {
		t.Errorf("Stat(%s) = %v, %v, want a file with mode 0600", s.File, fi.Mode(), err)
	}
}

func TestSaveFormat(t *testing.T) {
	s := store(t)
	if err := s.Save(advice.UserRules{Protected: []string{"/a"}, Names: []string{"x&y"}}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	want := "{\n  \"Protected\": [\n    \"/a\"\n  ],\n  \"Recommended\": [],\n  \"Names\": [\n    \"x&y\"\n  ]\n}\n"
	if got := readFile(t, s.File); got != want {
		t.Fatalf("rules file = %q, want %q", got, want)
	}
}

func TestSaveAtomic(t *testing.T) {
	t.Run("overwrite leaves only rules.json", func(t *testing.T) {
		s := store(t)
		second := advice.UserRules{Protected: []string{"~"}, Names: []string{"second"}}
		for _, r := range []advice.UserRules{{Names: []string{"first"}}, second} {
			if err := s.Save(r); err != nil {
				t.Fatalf("Save(%+v): %v", r, err)
			}
		}
		got, err := s.Load()
		if err != nil || !slices.Equal(got.Protected, second.Protected) || !slices.Equal(got.Names, second.Names) {
			t.Errorf("Load() after two saves = %+v, %v, want %+v", got, err, second)
		}
		if names := dirNames(t, filepath.Dir(s.File)); !slices.Equal(names, []string{"rules.json"}) {
			t.Errorf("dir after two saves holds %v, want [rules.json] only", names)
		}
	})
	t.Run("parent is a regular file", func(t *testing.T) {
		dir := t.TempDir()
		parent := filepath.Join(dir, "notadir")
		if err := os.WriteFile(parent, []byte("keep"), 0o600); err != nil {
			t.Fatal(err)
		}
		s := userrules.Store{File: filepath.Join(parent, "rules.json")}
		if err := s.Save(advice.UserRules{Names: []string{"x"}}); err == nil {
			t.Fatal("Save beneath a regular file = nil, want error")
		}
		if got := readFile(t, parent); got != "keep" {
			t.Errorf("parent file = %q after failed Save, want unchanged \"keep\"", got)
		}
		if names := dirNames(t, dir); !slices.Equal(names, []string{"notadir"}) {
			t.Errorf("dir after failed Save holds %v, want [notadir] only", names)
		}
	})
	t.Run("unwritable dir keeps the target", func(t *testing.T) {
		if runtime.GOOS == "windows" || os.Geteuid() == 0 {
			t.Skip("directory write permission is not enforced here")
		}
		s := store(t)
		if err := s.Save(advice.UserRules{Names: []string{"old"}}); err != nil {
			t.Fatalf("Save: %v", err)
		}
		before := readFile(t, s.File)
		dir := filepath.Dir(s.File)
		if err := os.Chmod(dir, 0o500); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })
		if err := s.Save(advice.UserRules{Names: []string{"new"}}); err == nil {
			t.Fatal("Save into an unwritable dir = nil, want error")
		}
		if got := readFile(t, s.File); got != before {
			t.Errorf("rules file = %q after failed Save, want unchanged %q", got, before)
		}
		if names := dirNames(t, dir); !slices.Equal(names, []string{"rules.json"}) {
			t.Errorf("dir after failed Save holds %v, want [rules.json] only", names)
		}
	})
}

func TestLoadCorrupt(t *testing.T) {
	for _, tc := range []struct{ name, body string }{
		{"empty", ""},
		{"truncated", "{"},
		{"array", "[]"},
		{"wrong type", `{"Names":"x"}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := store(t)
			if err := os.WriteFile(s.File, []byte(tc.body), 0o600); err != nil {
				t.Fatal(err)
			}
			got, err := s.Load()
			if err == nil || !reflect.DeepEqual(got, zero) {
				t.Fatalf("Load() of %q = %+v, %v, want zero UserRules and an error", tc.body, got, err)
			}
			if msg := err.Error(); !strings.HasPrefix(msg, "userrules: ") || !strings.Contains(msg, s.File) {
				t.Errorf("Load() error = %q, want prefix \"userrules: \" and the path %q", msg, s.File)
			}
		})
	}
	t.Run("unknown keys ignored", func(t *testing.T) {
		s := store(t)
		if err := os.WriteFile(s.File, []byte(`{"Protected":["/a"],"X":1}`), 0o600); err != nil {
			t.Fatal(err)
		}
		got, err := s.Load()
		if err != nil || !slices.Equal(got.Protected, []string{"/a"}) || len(got.Recommended)+len(got.Names) != 0 {
			t.Fatalf("Load() = %+v, %v, want Protected [/a] only and nil", got, err)
		}
	})
}

func TestLoadReadErrorNamesPath(t *testing.T) {
	s := userrules.Store{File: t.TempDir()} // a directory cannot be read as a file
	got, err := s.Load()
	if err == nil || !reflect.DeepEqual(got, zero) {
		t.Fatalf("Load() of a directory = %+v, %v, want zero UserRules and an error", got, err)
	}
	if msg := err.Error(); !strings.HasPrefix(msg, "userrules: ") || !strings.Contains(msg, s.File) {
		t.Errorf("Load() error = %q, want prefix \"userrules: \" and the path %q", msg, s.File)
	}
}

func TestEmptyPath(t *testing.T) {
	var s userrules.Store
	if got := s.Path(); got != "" {
		t.Errorf("Store{}.Path() = %q, want \"\"", got)
	}
	got, err := s.Load()
	if !errors.Is(err, userrules.ErrNoPath) || !reflect.DeepEqual(got, zero) {
		t.Errorf("Store{}.Load() = %+v, %v, want zero UserRules and ErrNoPath", got, err)
	}
	cwd := dirNames(t, ".")
	if err := s.Save(advice.UserRules{Names: []string{"x"}}); !errors.Is(err, userrules.ErrNoPath) {
		t.Errorf("Store{}.Save() = %v, want ErrNoPath", err)
	}
	if after := dirNames(t, "."); !slices.Equal(after, cwd) {
		t.Errorf("working directory after Store{}.Save() holds %v, want unchanged %v", after, cwd)
	}
}

func TestValidate(t *testing.T) {
	const (
		notAbs   = "not absolute or ~-relative"
		reserved = "reserved name"
		hasSep   = "contains a path separator"
	)
	abs := filepath.Join(t.TempDir(), "x")
	// broad paths name a whole home or volume: fine as Protected, too broad as Recommended (TestValidateRejectsBroadRecommended).
	broad := []string{"~", "~/"}
	validPaths := []string{"~/x", "~" + sep + "x", " ~/padded ", abs, abs + sep, " " + abs}
	invalidPaths := []string{"a", "./a", "../a", "~user", "~~", "~ /x", "x/~", "a" + sep + "b"}
	if runtime.GOOS == "windows" {
		broad = append(broad, `c:\`, `\\srv\share`)
		validPaths = append(validPaths, `C:\x`, `C:/x`, `\\srv\share\x`, `~\x`)
		invalidPaths = append(invalidPaths, `\x`, `/x`, `C:x`, `C:`)
	} else {
		broad = append(broad, "/")
		validPaths = append(validPaths, "/a", "/a/b/", "//a")
		invalidPaths = append(invalidPaths, `C:\x`, `\\srv\share`, `~\x`, `\x`)
	}
	validNames := []string{"node_modules", ".cache", "a b", " padded ", "~", "...", "-"}
	invalidNames := []struct{ name, reason string }{
		{".", reserved}, {"..", reserved}, {" . ", reserved}, {"\t..\n", reserved},
		{"a/b", hasSep}, {`a\b`, hasSep}, {"/", hasSep}, {`\`, hasSep}, {"a/", hasSep}, {"/a", hasSep}, {"./", hasSep}, {"../a", hasSep},
	}

	for _, list := range []string{"Protected", "Recommended"} {
		valid := validPaths
		if list == "Protected" {
			valid = slices.Concat(validPaths, broad)
		}
		for _, p := range valid {
			if err := userrules.Validate(oneRule(list, p)); err != nil {
				t.Errorf("Validate(%s %q) = %v, want nil", list, p, err)
			}
		}
		for _, p := range invalidPaths {
			err := userrules.Validate(oneRule(list, p))
			want := "userrules: invalid rules:\n" + list + "[0]: " + notAbs + ": " + strings.TrimSpace(p)
			if !errors.Is(err, userrules.ErrInvalidRules) || err.Error() != want {
				t.Errorf("Validate(%s %q) = %v, want ErrInvalidRules reading %q", list, p, err, want)
			}
		}
	}
	for _, n := range validNames {
		if err := userrules.Validate(oneRule("Names", n)); err != nil {
			t.Errorf("Validate(Names %q) = %v, want nil", n, err)
		}
	}
	for _, tc := range invalidNames {
		err := userrules.Validate(oneRule("Names", tc.name))
		want := "userrules: invalid rules:\nNames[0]: " + tc.reason + ": " + strings.TrimSpace(tc.name)
		if !errors.Is(err, userrules.ErrInvalidRules) || err.Error() != want {
			t.Errorf("Validate(Names %q) = %v, want ErrInvalidRules reading %q", tc.name, err, want)
		}
	}

	t.Run("blank items are ignored", func(t *testing.T) {
		r := advice.UserRules{Protected: []string{"", " ", "\t\n"}, Recommended: []string{""}, Names: []string{" ", ""}}
		if err := userrules.Validate(r); err != nil {
			t.Errorf("Validate(%+v) = %v, want nil", r, err)
		}
		if err := userrules.Validate(advice.UserRules{}); err != nil {
			t.Errorf("Validate(zero) = %v, want nil", err)
		}
	})
	t.Run("aggregates every violation", func(t *testing.T) {
		r := advice.UserRules{
			Protected:   []string{" ", "rel", "~", " ./x "},
			Recommended: []string{"~/fine", " also/rel "},
			Names:       []string{"ok", ".", "a/b", " .. ", `c\d`, ""},
		}
		want := strings.Join([]string{
			"userrules: invalid rules:",
			"Protected[1]: " + notAbs + ": rel",
			"Protected[3]: " + notAbs + ": ./x",
			"Recommended[1]: " + notAbs + ": also/rel",
			"Names[1]: " + reserved + ": .",
			"Names[2]: " + hasSep + ": a/b",
			"Names[3]: " + reserved + ": ..",
			"Names[4]: " + hasSep + `: c\d`,
		}, "\n")
		err := userrules.Validate(r)
		if !errors.Is(err, userrules.ErrInvalidRules) {
			t.Fatalf("Validate(%+v) = %v, want ErrInvalidRules", r, err)
		}
		if got := err.Error(); got != want {
			t.Errorf("Validate(%+v) error =\n%s\nwant\n%s", r, got, want)
		}
	})
	t.Run("normalized valid rules validate", func(t *testing.T) {
		for _, r := range []advice.UserRules{
			{},
			{Protected: []string{" ~ ", "~", "~/x", ""}, Recommended: []string{"~/y", " "}, Names: []string{"a", " a ", "b"}},
			{Protected: []string{abs}, Recommended: []string{abs + sep, " " + abs}, Names: []string{".cache", "node_modules"}},
		} {
			if err := userrules.Validate(userrules.Normalize(r)); err != nil {
				t.Errorf("Validate(Normalize(%+v)) = %v, want nil", r, err)
			}
		}
	})
}

func TestValidateRejectsBroadRecommended(t *testing.T) {
	hostRoot := filepath.VolumeName(t.TempDir()) + sep
	// reason "" means the path is acceptable as Recommended.
	type pathReason struct{ path, reason string }
	cases := []pathReason{
		{"~", "too broad"},
		{"~/", "too broad"},
		{"~//", "too broad"},
		{"~/.", "too broad"},
		{"~/./", "too broad"},
		{"~/..", "too broad"},
		{"~/x/..", "too broad"},
		{"~/x/../", "too broad"},
		{hostRoot, "too broad"},
		{hostRoot + "x", ""},
		{"~/x", ""},
		{"~/.cache", ""},
		{"~/x/y/..", ""},
	}
	if runtime.GOOS == "windows" {
		cases = append(cases, pathReason{`~\`, "too broad"}, pathReason{`C:\`, "too broad"}, pathReason{`\\srv\share`, "too broad"})
	} else {
		cases = append(cases, pathReason{`~\`, "not absolute or ~-relative"})
	}
	for _, tc := range cases {
		err := userrules.Validate(oneRule("Recommended", tc.path))
		if tc.reason == "" {
			if err != nil {
				t.Errorf("Validate(Recommended %q) = %v, want nil", tc.path, err)
			}
			continue
		}
		want := "userrules: invalid rules:\nRecommended[0]: " + tc.reason + ": " + tc.path
		if !errors.Is(err, userrules.ErrInvalidRules) || err.Error() != want {
			t.Errorf("Validate(Recommended %q) = %v, want ErrInvalidRules reading %q", tc.path, err, want)
		}
	}
}

func TestValidateBroadOnlyForRecommended(t *testing.T) {
	hostRoot := filepath.VolumeName(t.TempDir()) + sep
	for _, p := range []string{"~", "~/", "~//", "~/.", hostRoot} {
		if err := userrules.Validate(oneRule("Recommended", p)); !errors.Is(err, userrules.ErrInvalidRules) || !strings.Contains(err.Error(), "too broad") {
			t.Errorf("Validate(Recommended %q) = %v, want ErrInvalidRules reading too broad", p, err)
		}
		if err := userrules.Validate(oneRule("Protected", p)); err != nil {
			t.Errorf("Validate(Protected %q) = %v, want nil: Protected may cover a whole home or volume", p, err)
		}
	}
	if err := userrules.Validate(oneRule("Names", "~")); err != nil {
		t.Errorf("Validate(Names %q) = %v, want nil: ~ is a plain name", "~", err)
	}
}

func TestValidateAggregatesBroadWithOthers(t *testing.T) {
	hostRoot := filepath.VolumeName(t.TempDir()) + sep
	r := advice.UserRules{Protected: []string{"rel"}, Recommended: []string{"~", hostRoot + "ok"}, Names: []string{"a/b"}}
	want := strings.Join([]string{
		"userrules: invalid rules:",
		"Protected[0]: not absolute or ~-relative: rel",
		"Recommended[0]: too broad: ~",
		"Names[0]: contains a path separator: a/b",
	}, "\n")
	err := userrules.Validate(r)
	if !errors.Is(err, userrules.ErrInvalidRules) {
		t.Fatalf("Validate(%+v) = %v, want ErrInvalidRules", r, err)
	}
	if got := err.Error(); got != want {
		t.Errorf("Validate(%+v) error =\n%s\nwant\n%s", r, got, want)
	}
}

func TestLoadDecodeErrorReturnsZero(t *testing.T) {
	s := store(t)
	if err := os.WriteFile(s.File, []byte(`{"Protected":["/a"],"Names":"x"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := s.Load()
	if err == nil || !strings.Contains(err.Error(), s.File) {
		t.Fatalf("Load() of a half-decodable file = %+v, %v, want an error naming %q", got, err, s.File)
	}
	if len(got.Protected)+len(got.Recommended)+len(got.Names) != 0 {
		t.Errorf("Load() of a half-decodable file = %+v, want no rules at all rather than the part that decoded", got)
	}
}

func TestSaveIntoDirectoryLeavesNoTemp(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "rules.json")
	if err := os.Mkdir(target, 0o755); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(target, "marker")
	if err := os.WriteFile(marker, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := (userrules.Store{File: target}).Save(advice.UserRules{Names: []string{"x"}}); err == nil {
		t.Fatal("Save onto a directory = nil, want error")
	}
	if fi, err := os.Stat(target); err != nil || !fi.IsDir() {
		t.Errorf("Stat(%s) = %v, %v after failed Save, want still a directory", target, fi, err)
	}
	if got := readFile(t, marker); got != "keep" {
		t.Errorf("file inside the directory = %q after failed Save, want unchanged \"keep\"", got)
	}
	if tmps, _ := filepath.Glob(filepath.Join(dir, ".rules.json.tmp-*")); len(tmps) != 0 {
		t.Errorf("failed Save left temporary files %v, want none", tmps)
	}
	if names := dirNames(t, dir); !slices.Equal(names, []string{"rules.json"}) {
		t.Errorf("dir after failed Save holds %v, want [rules.json] only", names)
	}
}

func TestNormalize(t *testing.T) {
	in := advice.UserRules{
		Protected:   []string{" /a ", "/a", "", "/a/", "~", "~/", "  ", "/b", "/b"},
		Recommended: []string{"~/x", " ~/x", "/A", "/a"},
		Names:       []string{"A", "a", " A ", "", "b"},
	}
	snapshot := advice.UserRules{
		Protected:   slices.Clone(in.Protected),
		Recommended: slices.Clone(in.Recommended),
		Names:       slices.Clone(in.Names),
	}
	want := advice.UserRules{
		Protected:   []string{"/a", "/a/", "~", "~/", "/b"},
		Recommended: []string{"~/x", "/A", "/a"},
		Names:       []string{"A", "a", "b"},
	}
	if got := userrules.Normalize(in); !reflect.DeepEqual(got, want) {
		t.Errorf("Normalize(%+v) = %+v, want %+v (trimmed, blanks dropped, first duplicate kept, nothing cleaned or folded)", in, got, want)
	}
	if !reflect.DeepEqual(in, snapshot) {
		t.Errorf("Normalize mutated its argument into %+v, want %+v", in, snapshot)
	}
	t.Run("empty is non-nil", func(t *testing.T) {
		got := userrules.Normalize(advice.UserRules{})
		if got.Protected == nil || got.Recommended == nil || got.Names == nil || len(got.Protected)+len(got.Recommended)+len(got.Names) != 0 {
			t.Errorf("Normalize(zero) = %#v, want three empty non-nil slices", got)
		}
	})
	t.Run("does not alias", func(t *testing.T) {
		in := advice.UserRules{Protected: []string{"/a"}, Recommended: []string{"/b"}, Names: []string{"c"}}
		got := userrules.Normalize(in)
		if !reflect.DeepEqual(got, in) {
			t.Fatalf("Normalize(%+v) = %+v, want it unchanged", in, got)
		}
		got.Protected[0], got.Recommended[0], got.Names[0] = "x", "y", "z"
		if in.Protected[0] != "/a" || in.Recommended[0] != "/b" || in.Names[0] != "c" {
			t.Errorf("writing to Normalize's result changed the argument to %+v, want slices that do not alias it", in)
		}
	})
}

// pathCase is a user path and where advice anchors it; an empty anchor means
// Validate must reject the path and advice must ignore it.
type pathCase struct{ path, anchor string }

func TestValidateMatchesAdvice(t *testing.T) {
	home := filepath.Join(t.TempDir(), "home")
	vol := filepath.VolumeName(home) + sep
	rules := func(u advice.UserRules) *advice.Rules {
		return advice.New(advice.Config{GOOS: "test", Home: home, Getenv: func(string) string { return "" }, User: u})
	}
	base := rules(advice.UserRules{})
	customP := advice.Verdict{Class: advice.Protected, Reason: advice.Custom}
	customR := advice.Verdict{Class: advice.Recommended, Reason: advice.Custom}

	paths := []pathCase{
		{"~", home},
		{"~" + sep + "keep", filepath.Join(home, "keep")},
		{"~/keep", filepath.Join(home, "keep")},
		{filepath.Join(home, "keep2"), filepath.Join(home, "keep2")},
		{"rel", ""},
		{"./x", ""},
		{"~user", ""},
	}
	if runtime.GOOS == "windows" {
		paths = append(paths, pathCase{`\x`, ""}, pathCase{`/x`, ""}, pathCase{`\\srv\share\x`, `\\srv\share\x`})
	} else {
		paths = append(paths, pathCase{`~\keep`, ""}, pathCase{`C:\x`, ""})
	}
	for _, tc := range paths {
		t.Run(tc.path, func(t *testing.T) {
			valid := tc.anchor != ""
			for _, list := range []string{"Protected", "Recommended"} {
				// Recommended refuses the whole home as too broad even though advice would anchor it.
				wantValid := valid && !(list == "Recommended" && tc.path == "~")
				if err := userrules.Validate(oneRule(list, tc.path)); (err == nil) != wantValid {
					t.Errorf("Validate(%s %q) = %v, want valid %v", list, tc.path, err, wantValid)
				}
			}
			protected := rules(advice.UserRules{Protected: []string{tc.path}})
			recommended := rules(advice.UserRules{Recommended: []string{tc.path}})
			if valid {
				probe := filepath.Join(tc.anchor, "probe")
				if got := protected.Classify(probe, false); got != customP {
					t.Errorf("Protected %q: Classify(%q) = %+v, want %+v", tc.path, probe, got, customP)
				}
				if got := recommended.Classify(probe, false); got != customR {
					t.Errorf("Recommended %q: Classify(%q) = %+v, want %+v", tc.path, probe, got, customR)
				}
				return
			}
			for _, probe := range []string{
				home,
				filepath.Join(home, "probe"),
				filepath.Join(home, strings.TrimLeft(tc.path, "~"), "probe"),
				filepath.Join(home, tc.path, "probe"),
				filepath.Join(vol, tc.path, "probe"),
			} {
				for _, dir := range []bool{false, true} {
					want := base.Classify(probe, dir)
					if got := protected.Classify(probe, dir); got != want {
						t.Errorf("Protected %q: Classify(%q, %v) = %+v, want %+v as without user rules", tc.path, probe, dir, got, want)
					}
					if got := recommended.Classify(probe, dir); got != want {
						t.Errorf("Recommended %q: Classify(%q, %v) = %+v, want %+v as without user rules", tc.path, probe, dir, got, want)
					}
				}
			}
		})
	}
	t.Run("names", func(t *testing.T) {
		for _, n := range []string{"target", ".cache2", "a b"} {
			if err := userrules.Validate(oneRule("Names", n)); err != nil {
				t.Errorf("Validate(Names %q) = %v, want nil", n, err)
			}
			probe := filepath.Join(home, "p", n)
			if got := rules(advice.UserRules{Names: []string{n}}).Classify(probe, true); got != customR {
				t.Errorf("Names %q: Classify(%q, dir) = %+v, want %+v", n, probe, got, customR)
			}
		}
		for _, n := range []string{".", "..", "a/b", `a\b`} {
			if err := userrules.Validate(oneRule("Names", n)); !errors.Is(err, userrules.ErrInvalidRules) {
				t.Errorf("Validate(Names %q) = %v, want ErrInvalidRules", n, err)
			}
			r := rules(advice.UserRules{Names: []string{n}})
			for _, probe := range []string{
				filepath.Join(home, "p", n),
				filepath.Join(home, "p", "a"),
				filepath.Join(home, "p", "b"),
				filepath.Join(home, "p", "a", "b"),
			} {
				if got, want := r.Classify(probe, true), base.Classify(probe, true); got != want {
					t.Errorf("Names %q: Classify(%q, dir) = %+v, want %+v as without user rules", n, probe, got, want)
				}
			}
		}
	})
}
