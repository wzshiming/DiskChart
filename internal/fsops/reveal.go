package fsops

import (
	"context"
	"net/url"
	"os"
	"os/exec"
	posixpath "path"
	"runtime"
	"strings"
)

var runCommand = func(ctx context.Context, name string, args ...string) error {
	return exec.CommandContext(ctx, name, args...).Run()
}

// Reveal shows path in the system file manager.
func Reveal(ctx context.Context, path string) error {
	fi, err := os.Stat(path)
	isDir := err == nil && fi.IsDir()
	return reveal(ctx, runtime.GOOS, path, isDir)
}

func reveal(ctx context.Context, goos, path string, isDir bool) error {
	var err error
	for _, cmd := range revealCommands(goos, path, isDir) {
		if err = runCommand(ctx, cmd[0], cmd[1:]...); err == nil {
			return nil
		}
	}
	// explorer.exe often exits non-zero even on success.
	if goos == "windows" {
		return nil
	}
	return err
}

func revealCommands(goos, path string, isDir bool) [][]string {
	dir := path
	if !isDir {
		// dir only feeds xdg-open, which runs on POSIX hosts.
		dir = posixpath.Dir(path)
	}
	switch goos {
	case "darwin":
		return [][]string{{"open", "-R", path}}
	case "windows":
		return [][]string{{"explorer.exe", "/select," + path}}
	case "linux":
		// dbus-send splits array:string: elements on commas.
		fileURL := strings.ReplaceAll((&url.URL{Scheme: "file", Path: path}).String(), ",", "%2C")
		return [][]string{
			{"dbus-send", "--session", "--print-reply", "--dest=org.freedesktop.FileManager1", "/org/freedesktop/FileManager1", "org.freedesktop.FileManager1.ShowItems", "array:string:" + fileURL, "string:"},
			{"xdg-open", dir},
		}
	default:
		return [][]string{{"xdg-open", dir}}
	}
}
