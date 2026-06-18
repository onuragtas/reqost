package watcher

import (
	"log"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

type Watcher struct {
	mu       sync.Mutex
	fsw      *fsnotify.Watcher
	watched  map[string]struct{}
	onChange func(path string)
	timer    *time.Timer
}

func New(onChange func(path string)) (*Watcher, error) {
	fsw, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	w := &Watcher{
		fsw:      fsw,
		watched:  make(map[string]struct{}),
		onChange: onChange,
	}
	go w.loop()
	return w, nil
}

func (w *Watcher) Watch(path string) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if _, ok := w.watched[path]; ok {
		return nil
	}
	if err := w.fsw.Add(path); err != nil {
		return err
	}
	w.watched[path] = struct{}{}
	return nil
}

func (w *Watcher) Close() error {
	return w.fsw.Close()
}

func (w *Watcher) loop() {
	for {
		select {
		case event, ok := <-w.fsw.Events:
			if !ok {
				return
			}
			if event.Has(fsnotify.Write) || event.Has(fsnotify.Create) {
				w.debounce(event.Name)
			}
		case err, ok := <-w.fsw.Errors:
			if !ok {
				return
			}
			log.Printf("watcher: %v", err)
		}
	}
}

func (w *Watcher) debounce(path string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.timer != nil {
		w.timer.Stop()
	}
	w.timer = time.AfterFunc(500*time.Millisecond, func() {
		w.onChange(path)
	})
}
