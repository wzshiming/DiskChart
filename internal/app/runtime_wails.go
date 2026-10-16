package app

import (
	"context"
	"errors"
	goruntime "runtime"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

var errNoRuntime = errors.New("app: wails runtime not started")

// WailsRuntime implements Runtime on top of the Wails runtime package.
type WailsRuntime struct {
	ctx context.Context
}

// Startup stores the Wails context; pass it as options.App.OnStartup.
func (r *WailsRuntime) Startup(ctx context.Context) { r.ctx = ctx }

// EventsEmit sends an event to the frontend; it is a no-op before Startup.
func (r *WailsRuntime) EventsEmit(name string, data any) {
	if r.ctx == nil {
		return
	}
	runtime.EventsEmit(r.ctx, name, data)
}

// OpenDirectoryDialog shows the native directory picker.
func (r *WailsRuntime) OpenDirectoryDialog(title string) (string, error) {
	if r.ctx == nil {
		return "", errNoRuntime
	}
	return runtime.OpenDirectoryDialog(r.ctx, runtime.OpenDialogOptions{Title: title})
}

// ClipboardSetText writes text to the system clipboard.
func (r *WailsRuntime) ClipboardSetText(text string) error {
	if r.ctx == nil {
		return errNoRuntime
	}
	return runtime.ClipboardSetText(r.ctx, text)
}

// Platform reports the host OS, falling back to GOOS before Startup.
func (r *WailsRuntime) Platform() string {
	if r.ctx == nil {
		return goruntime.GOOS
	}
	return runtime.Environment(r.ctx).Platform
}
