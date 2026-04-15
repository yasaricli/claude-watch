// Package watcher provides file-system watching with a polling fallback,
// used to detect changes to Claude Code session JSONL files.
package watcher

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
)

// Watcher wraps fsnotify with automatic polling fallback and debouncing.
type Watcher struct {
	fw       *fsnotify.Watcher
	interval time.Duration
	trigger  chan struct{}
	Events   chan struct{} // fires when a refresh is needed
	done     chan struct{}
}

// New creates a Watcher for the given directories and their .jsonl files.
// Falls back to pure polling if fsnotify is unavailable.
func New(dirs []string, interval time.Duration) (*Watcher, error) {
	fw, err := fsnotify.NewWatcher()
	if err != nil {
		return pollingOnly(interval), nil
	}
	w := &Watcher{
		fw:       fw,
		interval: interval,
		trigger:  make(chan struct{}, 1),
		Events:   make(chan struct{}, 1),
		done:     make(chan struct{}),
	}
	for _, d := range dirs {
		w.addDir(d)
	}
	go w.run()
	return w, nil
}

func pollingOnly(interval time.Duration) *Watcher {
	w := &Watcher{
		interval: interval,
		Events:   make(chan struct{}, 1),
		done:     make(chan struct{}),
	}
	go func() {
		t := time.NewTicker(interval)
		defer t.Stop()
		for {
			select {
			case <-t.C:
				select {
				case w.Events <- struct{}{}:
				default:
				}
			case <-w.done:
				return
			}
		}
	}()
	return w
}

func (w *Watcher) addDir(dir string) {
	if w.fw == nil {
		return
	}
	_ = w.fw.Add(dir)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".jsonl") {
			_ = w.fw.Add(filepath.Join(dir, e.Name()))
		}
	}
}

func (w *Watcher) run() {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	fire := func() {
		select {
		case w.trigger <- struct{}{}:
		default:
		}
	}

	for {
		select {
		case <-w.done:
			return
		case ev, ok := <-w.fw.Events:
			if !ok {
				return
			}
			if ev.Has(fsnotify.Write) || ev.Has(fsnotify.Create) {
				if ev.Has(fsnotify.Create) && strings.HasSuffix(ev.Name, ".jsonl") {
					_ = w.fw.Add(ev.Name)
				}
				fire()
			}
		case <-ticker.C:
			fire()
		case <-w.trigger:
			time.Sleep(300 * time.Millisecond)
			select {
			case w.Events <- struct{}{}:
			default:
			}
		case _, ok := <-w.fw.Errors:
			if !ok {
				return
			}
		}
	}
}

// Close stops the watcher.
func (w *Watcher) Close() {
	close(w.done)
	if w.fw != nil {
		w.fw.Close()
	}
}
