// Package userrules keeps the user's own advice rules in a JSON file and checks them before they are applied.
package userrules

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/wzshiming/DiskChart/internal/advice"
)

// Errors returned by this package.
var (
	ErrInvalidRules = errors.New("userrules: invalid rules")
	ErrNoPath       = errors.New("userrules: no config file path")
)

// DefaultPath returns the rules file beneath the user's configuration directory.
func DefaultPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "DiskChart", "rules.json"), nil
}

// Store reads and writes the rules file File; "" means there is none.
type Store struct {
	File string
}

// Path returns the rules file.
func (s Store) Path() string { return s.File }

// Load reads the rules exactly as stored, ignoring unknown keys: a missing file yields zero rules and no
// error, while an unreadable or malformed one yields zero rules and an error naming the file.
func (s Store) Load() (advice.UserRules, error) {
	if s.File == "" {
		return advice.UserRules{}, ErrNoPath
	}
	b, err := os.ReadFile(s.File)
	if errors.Is(err, fs.ErrNotExist) {
		return advice.UserRules{}, nil
	}
	if err != nil {
		return advice.UserRules{}, fmt.Errorf("userrules: %w", err)
	}
	var r advice.UserRules
	if err := json.Unmarshal(b, &r); err != nil {
		return advice.UserRules{}, fmt.Errorf("userrules: parse %s: %w", s.File, err)
	}
	return r, nil
}

// Save replaces the rules file with r, unchecked, through a temporary file in the same directory;
// on failure the old file is untouched and the temporary file removed.
func (s Store) Save(r advice.UserRules) error {
	if s.File == "" {
		return ErrNoPath
	}
	if err := s.save(r); err != nil {
		return fmt.Errorf("userrules: save %s: %w", s.File, err)
	}
	return nil
}

func (s Store) save(r advice.UserRules) error {
	dir := filepath.Dir(s.File)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, ".rules.json.tmp-*")
	if err != nil {
		return err
	}
	if err := write(f, r); err != nil {
		os.Remove(f.Name())
		return err
	}
	if err := os.Rename(f.Name(), s.File); err != nil {
		os.Remove(f.Name())
		return err
	}
	return nil
}

// write restricts f to its owner, encodes r into it, syncs and closes it.
func write(f *os.File, r advice.UserRules) (err error) {
	defer func() {
		if cerr := f.Close(); err == nil {
			err = cerr
		}
	}()
	if err := f.Chmod(0o600); err != nil {
		return err
	}
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	if err := enc.Encode(nonNil(r)); err != nil {
		return err
	}
	return f.Sync()
}

// nonNil replaces nil slices with empty ones so they encode as [] rather than null.
func nonNil(r advice.UserRules) advice.UserRules {
	if r.Protected == nil {
		r.Protected = []string{}
	}
	if r.Recommended == nil {
		r.Recommended = []string{}
	}
	if r.Names == nil {
		r.Names = []string{}
	}
	return r
}

// Validate reports every entry the rules could not use, trimmed and with blank entries ignored: paths must be
// absolute or ~ alone or ~ followed by a separator, and names must be single path segments other than . and ..
// The error wraps ErrInvalidRules and lists one offender per line as Field[i]: reason: value.
func Validate(r advice.UserRules) error {
	var lines []string
	collect := func(field string, items []string, fault func(string) string) {
		for i, s := range items {
			s = strings.TrimSpace(s)
			if s == "" {
				continue
			}
			if reason := fault(s); reason != "" {
				lines = append(lines, fmt.Sprintf("%s[%d]: %s: %s", field, i, reason, s))
			}
		}
	}
	collect("Protected", r.Protected, pathFault)
	collect("Recommended", r.Recommended, pathFault)
	collect("Names", r.Names, nameFault)
	if lines == nil {
		return nil
	}
	return fmt.Errorf("%w:\n%s", ErrInvalidRules, strings.Join(lines, "\n"))
}

// pathFault says why p is neither absolute nor ~-relative, or "" when it is one of them.
func pathFault(p string) string {
	if p == "~" || (len(p) > 1 && p[0] == '~' && os.IsPathSeparator(p[1])) || filepath.IsAbs(p) {
		return ""
	}
	return "not absolute or ~-relative"
}

// nameFault says why n cannot be a directory base name, or "" when it can.
func nameFault(n string) string {
	switch {
	case n == "." || n == "..":
		return "reserved name"
	case strings.ContainsAny(n, `/\`):
		return "contains a path separator"
	}
	return ""
}

// Normalize trims every entry and drops blank and repeated ones, keeping the first of each; the result
// shares no slice with r and has none nil. Paths are neither cleaned nor expanded.
func Normalize(r advice.UserRules) advice.UserRules {
	return advice.UserRules{
		Protected:   normalizeList(r.Protected),
		Recommended: normalizeList(r.Recommended),
		Names:       normalizeList(r.Names),
	}
}

func normalizeList(in []string) []string {
	out := make([]string, 0, len(in))
	seen := make(map[string]bool, len(in))
	for _, s := range in {
		s = strings.TrimSpace(s)
		if s == "" || seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	return out
}
