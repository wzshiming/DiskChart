package fsops

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"testing"
)

type recorder struct {
	calls  [][]string
	nilCtx bool
}

// stubRunCommand replaces runCommand; result(i) decides the error of the i-th call.
func stubRunCommand(t *testing.T, result func(i int) error) *recorder {
	t.Helper()
	r := &recorder{}
	orig := runCommand
	t.Cleanup(func() { runCommand = orig })
	runCommand = func(ctx context.Context, name string, args ...string) error {
		if ctx == nil {
			r.nilCtx = true
		}
		i := len(r.calls)
		r.calls = append(r.calls, append([]string{name}, args...))
		return result(i)
	}
	return r
}

func ok(int) error { return nil }

const linuxFile = "/tmp/a b/f.txt"

var linuxFileDbus = []string{
	"dbus-send", "--session", "--print-reply",
	"--dest=org.freedesktop.FileManager1",
	"/org/freedesktop/FileManager1",
	"org.freedesktop.FileManager1.ShowItems",
	"array:string:file:///tmp/a%20b/f.txt",
	"string:",
}

func assertCommands(t *testing.T, got, want [][]string) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("commands = %q, want %q", got, want)
	}
}

func TestRevealCommandsDarwinUsesOpenReveal(t *testing.T) {
	assertCommands(t, revealCommands("darwin", linuxFile, false),
		[][]string{{"open", "-R", linuxFile}})
}

func TestRevealCommandsWindowsUsesExplorerSelect(t *testing.T) {
	p := `C:\Users\a b\f.txt`
	assertCommands(t, revealCommands("windows", p, false),
		[][]string{{"explorer.exe", "/select," + p}})
}

func TestRevealCommandsLinuxFileUsesDbusThenXdgOpenParent(t *testing.T) {
	assertCommands(t, revealCommands("linux", linuxFile, false),
		[][]string{linuxFileDbus, {"xdg-open", "/tmp/a b"}})
}

func TestRevealCommandsLinuxEscapesComma(t *testing.T) {
	got := revealCommands("linux", "/tmp/a,b/f.txt", false)[0]
	if want := "array:string:file:///tmp/a%2Cb/f.txt"; !slices.Contains(got, want) {
		t.Fatalf("dbus-send args = %q, want file URL arg %q (comma escaped)", got, want)
	}
}

func TestRevealCommandsLinuxDirFallbackOpensDirItself(t *testing.T) {
	got := revealCommands("linux", "/tmp/d", true)
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2: %q", len(got), got)
	}
	if got[0][0] != "dbus-send" {
		t.Fatalf("first command = %q, want dbus-send", got[0])
	}
	if want := []string{"xdg-open", "/tmp/d"}; !reflect.DeepEqual(got[1], want) {
		t.Fatalf("second command = %q, want %q", got[1], want)
	}
}

func TestRevealCommandsOtherOSOnlyXdgOpen(t *testing.T) {
	assertCommands(t, revealCommands("freebsd", linuxFile, false),
		[][]string{{"xdg-open", "/tmp/a b"}})
}

func TestRevealDarwinRunsOpenOnce(t *testing.T) {
	r := stubRunCommand(t, ok)
	if err := reveal(context.Background(), "darwin", linuxFile, false); err != nil {
		t.Fatalf("reveal = %v", err)
	}
	assertCommands(t, r.calls, [][]string{{"open", "-R", linuxFile}})
}

func TestRevealLinuxFallsBackToXdgOpenWhenDbusFails(t *testing.T) {
	r := stubRunCommand(t, func(i int) error {
		if i == 0 {
			return errors.New("dbus failed")
		}
		return nil
	})
	if err := reveal(context.Background(), "linux", linuxFile, false); err != nil {
		t.Fatalf("reveal = %v, want nil after fallback", err)
	}
	assertCommands(t, r.calls, [][]string{linuxFileDbus, {"xdg-open", "/tmp/a b"}})
}

func TestRevealLinuxReturnsErrorWhenAllCommandsFail(t *testing.T) {
	r := stubRunCommand(t, func(int) error { return errors.New("fail") })
	if err := reveal(context.Background(), "linux", linuxFile, false); err == nil {
		t.Fatal("reveal = nil, want error when every command fails")
	}
	if len(r.calls) != 2 {
		t.Fatalf("calls = %d, want 2: %q", len(r.calls), r.calls)
	}
}

func TestRevealWindowsIgnoresExplorerError(t *testing.T) {
	p := `C:\x\f.txt`
	r := stubRunCommand(t, func(int) error { return errors.New("exit status 1") })
	if err := reveal(context.Background(), "windows", p, false); err != nil {
		t.Fatalf("reveal = %v, want nil (explorer exit code is unreliable)", err)
	}
	assertCommands(t, r.calls, [][]string{{"explorer.exe", "/select," + p}})
}

func TestRevealPassesNonNilContext(t *testing.T) {
	r := stubRunCommand(t, ok)
	_ = reveal(context.Background(), "darwin", linuxFile, false)
	if len(r.calls) == 0 {
		t.Fatal("runCommand not called")
	}
	if r.nilCtx {
		t.Fatal("runCommand received nil ctx")
	}
}

func TestRevealPublicUsesCurrentOSAndDirFlag(t *testing.T) {
	dir := t.TempDir()
	r := stubRunCommand(t, ok)
	_ = Reveal(context.Background(), dir)
	if len(r.calls) == 0 {
		t.Fatal("runCommand not called")
	}
	want := revealCommands(runtime.GOOS, dir, true)[0]
	if !reflect.DeepEqual(r.calls[0], want) {
		t.Fatalf("first command = %q, want %q", r.calls[0], want)
	}
}

func TestRevealPublicMissingPathTreatedAsFile(t *testing.T) {
	p := filepath.Join(t.TempDir(), "missing", "f.txt")
	if _, err := os.Lstat(p); err == nil {
		t.Fatal("precondition: path must not exist")
	}
	r := stubRunCommand(t, ok)
	_ = Reveal(context.Background(), p)
	if len(r.calls) == 0 {
		t.Fatal("runCommand not called")
	}
	want := revealCommands(runtime.GOOS, p, false)[0]
	if !reflect.DeepEqual(r.calls[0], want) {
		t.Fatalf("first command = %q, want %q", r.calls[0], want)
	}
}
