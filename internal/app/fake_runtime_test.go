package app

import "sync"

type fakeEvent struct {
	Name string
	Data any
}

type fakeRuntime struct {
	mu       sync.Mutex
	events   []fakeEvent
	dir      string
	dirErr   error
	dirTitle string
	clips    []string
	platform string
	finished chan struct{}
	closed   bool
}

func newFakeRuntime() *fakeRuntime {
	return &fakeRuntime{platform: "testos", finished: make(chan struct{})}
}

func (r *fakeRuntime) EventsEmit(name string, data any) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, fakeEvent{name, data})
	if name == EventScanFinished && !r.closed {
		r.closed = true
		close(r.finished)
	}
}

func (r *fakeRuntime) OpenDirectoryDialog(title string) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.dirTitle = title
	return r.dir, r.dirErr
}

func (r *fakeRuntime) ClipboardSetText(text string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.clips = append(r.clips, text)
	return nil
}

func (r *fakeRuntime) Platform() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.platform
}

func (r *fakeRuntime) eventsNamed(name string) []fakeEvent {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []fakeEvent
	for _, e := range r.events {
		if e.Name == name {
			out = append(out, e)
		}
	}
	return out
}
