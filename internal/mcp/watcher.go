package mcp

import (
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// FileChangeHandler is called when a .go file changes.
type FileChangeHandler func(path string)

// Watcher watches a directory tree for .go file changes and triggers callbacks.
type Watcher struct {
	rootDir  string
	watcher  *fsnotify.Watcher
	handler  FileChangeHandler
	logger   *log.Logger
	stopCh   chan struct{}
	wg       sync.WaitGroup
	debounce time.Duration
}

// NewWatcher creates a new file watcher for .go files.
func NewWatcher(rootDir string, handler FileChangeHandler, logger *log.Logger) (*Watcher, error) {
	fsw, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	w := &Watcher{
		rootDir:  rootDir,
		watcher:  fsw,
		handler:  handler,
		logger:   logger,
		stopCh:   make(chan struct{}),
		debounce: 100 * time.Millisecond,
	}

	// Walk the directory tree and add all directories to the watcher.
	err = filepath.Walk(rootDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			name := info.Name()
			if name == "vendor" || name == "node_modules" || name == ".git" || name == "bin" {
				return filepath.SkipDir
			}

			if addErr := fsw.Add(path); addErr != nil {
				w.logger.Printf("Warning: could not watch %s: %v", path, addErr)
			}

			return nil
		}

		return nil
	})
	if err != nil {
		fsw.Close()

		return nil, err
	}

	return w, nil
}

// Start begins watching for file changes in a background goroutine.
func (w *Watcher) Start() {
	w.wg.Add(1)

	go w.run()
}

// Stop stops the file watcher and waits for the background goroutine to finish.
func (w *Watcher) Stop() {
	close(w.stopCh)
	w.watcher.Close()
	w.wg.Wait()
}

func (w *Watcher) run() {
	defer w.wg.Done()

	// Debounce: collect changes over the debounce window, then fire once per file.
	pending := make(map[string]time.Time)
	ticker := time.NewTicker(50 * time.Millisecond)

	defer ticker.Stop()

	for {
		select {
		case <-w.stopCh:
			return

		case event, ok := <-w.watcher.Events:
			if !ok {
				return
			}

			if !isGoFile(event.Name) {
				// Check if a new directory was created — add it to the watcher.
				if event.Has(fsnotify.Create) {
					if info, err := os.Stat(event.Name); err == nil && info.IsDir() {
						name := info.Name()
						if name != "vendor" && name != "node_modules" && name != ".git" && name != "bin" {
							_ = w.watcher.Add(event.Name)
						}
					}
				}

				continue
			}

			if event.Has(fsnotify.Write) || event.Has(fsnotify.Create) || event.Has(fsnotify.Remove) || event.Has(fsnotify.Rename) {
				absPath, err := filepath.Abs(event.Name)
				if err != nil {
					absPath = event.Name
				}

				pending[absPath] = time.Now()
			}

		case err, ok := <-w.watcher.Errors:
			if !ok {
				return
			}

			w.logger.Printf("Watcher error: %v", err)

		case now := <-ticker.C:
			for path, ts := range pending {
				if now.Sub(ts) >= w.debounce {
					delete(pending, path)
					w.logger.Printf("File changed: %s", path)
					w.handler(path)
				}
			}
		}
	}
}

func isGoFile(path string) bool {
	return strings.HasSuffix(path, ".go") && !strings.HasSuffix(path, "_test.go")
}
