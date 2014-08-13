// Copyright 2014 Bowery, Inc.
package main

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Bowery/gopackages/ignores"
	"github.com/Bowery/gopackages/log"
	"github.com/Bowery/gopackages/tar"
)

// Event describes a file event and the associated application.
type Event struct {
	Application *Application `json:"application"`
	Status      string       `json:"status"`
	Path        string       `json:"path"`
}

// WatchError wraps an error to identify the app origin.
type WatchError struct {
	Application *Application `json:"application"`
	Err         error        `json:"error"`
}

func (w *WatchError) Error() string {
	return w.Err.Error()
}

// Watcher syncs file changes for an application to it's remote address.
type Watcher struct {
	Application *Application
	uploadPath  string
	done        chan struct{}
}

// NewWatcher creates a watcher.
func NewWatcher(app *Application) *Watcher {
	return &Watcher{
		Application: app,
		uploadPath:  filepath.Join(os.TempDir(), "bowery_"+app.ID),
		done:        make(chan struct{}),
	}
}

// Start syncs file changes and uploads to the applications remote address.
func (watcher *Watcher) Start(evChan chan *Event, errChan chan error) {
	stats := make(map[string]os.FileInfo)
	found := make([]string, 0)
	local := watcher.Application.LocalPath

	ignoreList, err := ignores.Get(local)
	if err != nil {
		errChan <- watcher.wrapErr(err)
		ignoreList = make([]string, 0)
	}

	// Get initial stats.
	err = filepath.Walk(local, func(path string, info os.FileInfo, err error) error {
		if err != nil {
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
			return err
		}

		rel, err := filepath.Rel(local, path)
		if err != nil {
			return err
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

		// Ignore directory changes, and no event status.
		if info.IsDir() || status == "" {
			stats[path] = info
			found = append(found, path)
			return nil
		}

		err = watcher.Update(rel, status)
		if err != nil {
			if os.IsNotExist(err) {
				log.Debug("Ignoring temp file", status, "event", rel)
				return nil
			}

			return err
		}

		evChan <- &Event{Application: watcher.Application, Status: status, Path: rel}
		stats[path] = info
		found = append(found, path)
		return nil
	}

	// Manages deletes.
	checkDeletes := func() error {
		for path := range stats {
			skip := false
			rel, err := filepath.Rel(local, path)
			if err != nil {
				return err
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
				return err
			}

			evChan <- &Event{Application: watcher.Application, Status: "delete", Path: rel}
		}

		return nil
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

		err = checkDeletes()
		if err != nil {
			errChan <- watcher.wrapErr(err)
		}

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
	return DelanceyUpdate(watcher.Application, filepath.Join(watcher.Application.LocalPath, name),
		name, status)
}

// Close closes the watcher and removes existing upload paths.
func (watcher *Watcher) Close() error {
	close(watcher.done)

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
func (syncer *Syncer) GetWatcher(app *Application) (*Watcher, error) {
	for _, watcher := range syncer.Watchers {
		if watcher != nil && watcher.Application.ID == app.ID {
			return watcher, nil
		}
	}

	return nil, errors.New("invalid app")
}

// Watch starts watching the given application syncing changes.
func (syncer *Syncer) Watch(app *Application) {
	watcher := NewWatcher(app)
	syncer.Watchers = append(syncer.Watchers, watcher)

	// Do the actual event management, and the inital upload.
	go func() {
		watcher.Start(syncer.Event, syncer.Error)
	}()
}

// Remove removes an applications syncer.
func (syncer *Syncer) Remove(app *Application) error {
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
