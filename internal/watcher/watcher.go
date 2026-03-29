// Package watcher implements file system watching for architecture validation.
package watcher

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/fsnotify/fsnotify"
)

// DefaultExcludes contains directory names that are always excluded from watching.
var DefaultExcludes = []string{
	"vendor", ".git", "node_modules", "testdata", "bin", ".claude",
}

// Watcher monitors Go source files for changes and triggers callbacks.
type Watcher struct {
	root     string
	excludes []string
	onChange func(changedFiles []string)
	watcher  *fsnotify.Watcher
}

// Option configures the Watcher.
type Option func(*Watcher)

// WithExcludes adds extra directory names to exclude.
func WithExcludes(dirs []string) Option {
	return func(w *Watcher) {
		w.excludes = append(w.excludes, dirs...)
	}
}

// New creates a new Watcher for the given root directory.
func New(root string, onChange func([]string), opts ...Option) (*Watcher, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolving root path: %w", err)
	}

	w := &Watcher{
		root:     absRoot,
		excludes: append([]string{}, DefaultExcludes...),
		onChange: onChange,
	}

	for _, opt := range opts {
		opt(w)
	}

	return w, nil
}

// Start begins watching for file changes. Blocks until ctx is cancelled.
func (w *Watcher) Start(ctx context.Context) error {
	fsw, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("creating fsnotify watcher: %w", err)
	}

	w.watcher = fsw

	defer func() {
		_ = fsw.Close()
	}()

	if err := w.addDirs(); err != nil {
		return fmt.Errorf("adding directories: %w", err)
	}

	debouncer := NewDebouncer(defaultDebounceMs, w.onChange)
	defer debouncer.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil

		case event, ok := <-fsw.Events:
			if !ok {
				return nil
			}

			if !w.isRelevant(event) {
				continue
			}

			// If a new directory was created, watch it too
			if event.Op&fsnotify.Create != 0 {
				w.tryAddDir(event.Name)
			}

			debouncer.FileChanged(event.Name)

		case err, ok := <-fsw.Errors:
			if !ok {
				return nil
			}

			fmt.Fprintf(os.Stderr, "watcher error: %v\n", err)
		}
	}
}

// addDirs recursively adds directories to the fsnotify watcher.
func (w *Watcher) addDirs() error {
	return filepath.WalkDir(w.root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil //nolint:nilerr // skip inaccessible dirs
		}

		if !d.IsDir() {
			return nil
		}

		if w.isExcluded(d.Name()) {
			return filepath.SkipDir
		}

		return w.watcher.Add(path)
	})
}

// tryAddDir adds a directory to the watcher if it's a new directory.
func (w *Watcher) tryAddDir(path string) {
	info, err := os.Stat(path)
	if err != nil || !info.IsDir() {
		return
	}

	if w.isExcluded(info.Name()) {
		return
	}

	_ = w.watcher.Add(path)
}

// isRelevant checks if a file system event is relevant (Go source file changed).
func (w *Watcher) isRelevant(event fsnotify.Event) bool {
	if event.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Rename) == 0 {
		return false
	}

	return isGoFile(event.Name)
}

// isExcluded checks if a directory name should be excluded.
func (w *Watcher) isExcluded(name string) bool {
	for _, exc := range w.excludes {
		if strings.EqualFold(name, exc) {
			return true
		}
	}

	return false
}

// isGoFile returns true if the file is a Go source file (not test).
func isGoFile(path string) bool {
	return strings.HasSuffix(path, ".go") && !strings.HasSuffix(path, "_test.go")
}
