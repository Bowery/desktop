// Copyright 2014 Bowery, Inc.
package main

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/Bowery/gopackages/ignores"
	"github.com/Bowery/gopackages/log"
	"github.com/Bowery/gopackages/schemas"
	"github.com/Bowery/gopackages/tar"
)

// Event describes a file event and the associated application.
type Event struct {
	Application *schemas.Application `json:"application"`
	Status      string               `json:"status"`
	Path        string               `json:"path"`
}

// WatchError wraps an error to identify the app origin.
type WatchError struct {
	Application *schemas.Application `json:"application"`
	Err         error                `json:"error"`
}

func (w *WatchError) Error() string {
	return w.Err.Error()
}

// Watcher syncs file changes for an application to it's remote address.
type Watcher struct {
	Application *schemas.Application
	uploadPath  string
	mutex       sync.Mutex
	done        chan struct{}
	isDone      bool
}

// NewWatcher creates a watcher.
func NewWatcher(app *schemas.Application) *Watcher {
	var mutex sync.Mutex

	return &Watcher{
		Application: app,
		uploadPath:  filepath.Join(os.TempDir(), "bowery_"+app.ID),
		mutex:       mutex,
		done:        make(chan struct{}),
	}
}

// Start syncs file changes and uploads to the applications remote address.
func (watcher *Watcher) Start(evChan chan *Event, errChan chan error) {
	stats := make(map[string]os.FileInfo)
	found := make([]string, 0)
	local := watcher.Application.LocalPath

	// If previously called Close reset the state.
	watcher.mutex.Lock()
	if watcher.isDone {
		watcher.isDone = false
		watcher.done = make(chan struct{})
	}
	watcher.mutex.Unlock()

	ignoreList, err := ignores.Get(local)
	if err != nil {
		errChan <- watcher.wrapErr(err)
		ignoreList = make([]string, 0)
	}

	// Get initial stats.
	err = filepath.Walk(local, func(path string, info os.FileInfo, err error) error {
		if err != nil || local == path {
			return err
		}

		// Check if ignoring.
		for _, ignore := range ignoreList {
			if ignore == path {
				if info.IsDir() {
					return filepath.SkipDir
				}

				return nil
			}
		}

		stats[path] = info
		return nil
	})
	if err != nil {
		errChan <- watcher.wrapErr(err)
	}

	// Manages updates/creates.
	walker := func(path string, info os.FileInfo, err error) error {
		if err != nil {
			errChan <- watcher.wrapErr(err)
			return nil
		}
		if local == path {
			return nil
		}

		rel, err := filepath.Rel(local, path)
		if err != nil {
			errChan <- watcher.wrapErr(err)
			return nil
		}

		// Check if ignoring.
		for _, ignore := range ignoreList {
			if ignore == path {
				for p := range stats {
					if p == path || strings.Contains(p, path+string(filepath.Separator)) {
						delete(stats, p)
					}
				}

				if info.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
		}
		pstat, ok := stats[path]
		status := ""

		// Check if created/updated.
		if ok && (info.ModTime().After(pstat.ModTime()) || info.Mode() != pstat.Mode()) {
			status = "update"
		} else if !ok {
			status = "create"
		}
		stats[path] = info
		found = append(found, path)

		// Ignore if no change has occured.
		if status == "" {
			return nil
		}

		err = watcher.Update(rel, status)
		if err != nil {
			if os.IsNotExist(err) {
				// Remove the stats info so we don't get a false delete later.
				delete(stats, path)
				found = found[:len(found)-1]
				log.Debug("Ignoring temp file", status, "event", rel)
				return nil
			}

			errChan <- watcher.wrapErr(err)
			return nil
		}

		evChan <- &Event{Application: watcher.Application, Status: status, Path: rel}
		return nil
	}

	// Manages deletes.
	checkDeletes := func() {
		for path := range stats {
			skip := false
			rel, err := filepath.Rel(local, path)
			if err != nil {
				errChan <- watcher.wrapErr(err)
				continue
			}

			for _, f := range found {
				if f == path {
					skip = true
					break
				}
			}

			if skip {
				continue
			}

			delete(stats, path)
			err = watcher.Update(rel, "delete")
			if err != nil {
				errChan <- watcher.wrapErr(err)
				continue
			}

			evChan <- &Event{Application: watcher.Application, Status: "delete", Path: rel}
		}
	}

	for {
		// Check if we're done.
		select {
		case <-watcher.done:
			return
		default:
		}

		ignoreList, err = ignores.Get(local)
		if err != nil {
			errChan <- watcher.wrapErr(err)
			ignoreList = make([]string, 0)
		}

		err = filepath.Walk(local, walker)
		if err != nil {
			errChan <- watcher.wrapErr(err)
		}

		checkDeletes()
		found = make([]string, 0)
		<-time.After(500 * time.Millisecond)
	}
}

// Upload compresses and uploads the contents to the applications remote address.
func (watcher *Watcher) Upload() error {
	var (
		err error
	)
	i := 0

	ignoreList, err := ignores.Get(watcher.Application.LocalPath)
	if err != nil {
		return watcher.wrapErr(err)
	}

	// Tar up the path and write it to the uploadPath.
	upload, err := tar.Tar(watcher.Application.LocalPath, ignoreList)
	if err != nil {
		return watcher.wrapErr(err)
	}

	err = os.MkdirAll(filepath.Dir(watcher.uploadPath), os.ModePerm|os.ModeDir)
	if err != nil {
		return watcher.wrapErr(err)
	}

	file, err := os.Create(watcher.uploadPath)
	if err != nil {
		return watcher.wrapErr(err)
	}
	defer os.RemoveAll(watcher.uploadPath)
	defer file.Close()

	_, err = io.Copy(file, upload)
	if err != nil {
		return watcher.wrapErr(err)
	}

	// Attempt to upload, ensuring the upload is at the beginning of the file.
	for i < 1000 {
		_, err = file.Seek(0, os.SEEK_SET)
		if err != nil {
			return watcher.wrapErr(err)
		}

		err = DelanceyUpload(watcher.Application, file)
		if err == nil {
			return nil
		}

		i++
		<-time.After(time.Millisecond * 50)
	}

	return watcher.wrapErr(err)
}

// Update updates a path to the applications remote address.
func (watcher *Watcher) Update(name, status string) error {
	path := filepath.Join(watcher.Application.LocalPath, name)

	err := DelanceyUpdate(watcher.Application, path, name, status)
	if err != nil && strings.Contains(err.Error(), "invalid app id") {
		// If the id is invalid that indicates the server died, just reupload
		// and try again.
		err = watcher.Upload()
		if err != nil {
			we, ok := err.(*WatchError)
			if ok {
				err = we.Err
			}

			return err
		}

		return DelanceyUpdate(watcher.Application, path, name, status)
	}

	return err
}

// Close closes the watcher and removes existing upload paths.
func (watcher *Watcher) Close() error {
	watcher.mutex.Lock()
	defer watcher.mutex.Unlock()

	if watcher.isDone {
		return nil
	}
	close(watcher.done)
	watcher.isDone = true

	return watcher.wrapErr(os.RemoveAll(watcher.uploadPath))
}

// wrapErr wraps an error with the application it occurred for.
func (watcher *Watcher) wrapErr(err error) error {
	if err == nil {
		return nil
	}

	return &WatchError{Application: watcher.Application, Err: err}
}

// Syncer manages the syncing of a list of file watchers.
type Syncer struct {
	Event    chan *Event
	Error    chan error
	Watchers []*Watcher
}

// NewSyncer creates a syncer.
func NewSyncer() *Syncer {
	return &Syncer{
		Event:    make(chan *Event),
		Error:    make(chan error),
		Watchers: make([]*Watcher, 0),
	}
}

// GetWatcher gets a watcher for a specific application.
func (syncer *Syncer) GetWatcher(app *schemas.Application) (*Watcher, bool) {
	for _, watcher := range syncer.Watchers {
		if watcher != nil && watcher.Application.ID == app.ID {
			return watcher, false
		}
	}

	return nil, true
}

// Watch starts watching the given application syncing changes.
func (syncer *Syncer) Watch(app *schemas.Application) {
	watcher := NewWatcher(app)
	syncer.Watchers = append(syncer.Watchers, watcher)

	// Do the actual event management, and the inital upload.
	go func() {
		syncer.Event <- &Event{Application: watcher.Application, Status: "upload-start"}
		err := watcher.Upload()
		if err != nil {
			syncer.Error <- err
			return
		}
		syncer.Event <- &Event{Application: watcher.Application, Status: "upload-finish"}

		watcher.Start(syncer.Event, syncer.Error)
	}()
}

// Remove removes an applications syncer.
func (syncer *Syncer) Remove(app *schemas.Application) error {
	for idx, watcher := range syncer.Watchers {
		if watcher != nil && watcher.Application.ID == app.ID {
			err := watcher.Close()
			if err != nil {
				return err
			}

			syncer.Watchers[idx] = nil
		}
	}

	return nil
}

// Close closes all the watchers.
func (syncer *Syncer) Close() error {
	for _, watcher := range syncer.Watchers {
		if watcher == nil {
			continue
		}

		err := watcher.Close()
		if err != nil {
			return err
		}
	}

	return nil
}
