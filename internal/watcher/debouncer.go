package watcher

import (
	"sync"
	"time"
)

const defaultDebounceMs = 500

// Debouncer batches rapid file changes into a single callback.
type Debouncer struct {
	delayMs  int
	callback func([]string)
	mu       sync.Mutex
	timer    *time.Timer
	files    map[string]struct{}
	stopped  bool
}

// NewDebouncer creates a Debouncer that waits delayMs after the last change
// before firing the callback with all accumulated file paths.
func NewDebouncer(delayMs int, callback func([]string)) *Debouncer {
	return &Debouncer{
		delayMs:  delayMs,
		callback: callback,
		files:    make(map[string]struct{}),
	}
}

// FileChanged records a file change. The callback fires delayMs after
// the last call to FileChanged.
func (d *Debouncer) FileChanged(path string) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.stopped {
		return
	}

	d.files[path] = struct{}{}

	if d.timer != nil {
		d.timer.Stop()
	}

	d.timer = time.AfterFunc(time.Duration(d.delayMs)*time.Millisecond, d.fire)
}

// Stop cancels any pending callback.
func (d *Debouncer) Stop() {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.stopped = true

	if d.timer != nil {
		d.timer.Stop()
	}
}

// fire is called by the timer. It collects accumulated files and invokes the callback.
func (d *Debouncer) fire() {
	d.mu.Lock()

	if d.stopped {
		d.mu.Unlock()
		return
	}

	files := make([]string, 0, len(d.files))
	for f := range d.files {
		files = append(files, f)
	}

	d.files = make(map[string]struct{})

	d.mu.Unlock()

	if len(files) > 0 {
		d.callback(files)
	}
}
