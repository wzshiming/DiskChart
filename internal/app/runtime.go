package app

// Runtime abstracts the Wails runtime so tests can inject a fake.
type Runtime interface {
	EventsEmit(name string, data any)
	OpenDirectoryDialog(title string) (string, error) // "" and nil when the user cancels
	ClipboardSetText(text string) error
	Platform() string // "darwin", "windows" or "linux"
}
