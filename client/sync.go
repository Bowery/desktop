// Copyright 2014 Bowery, Inc.
package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/Bowery/delancey/delancey"
	"github.com/Bowery/gopackages/config"
	"github.com/Bowery/gopackages/schemas"
	"github.com/Bowery/gopackages/tar"
	"github.com/Bowery/gopackages/util"
	"github.com/Bowery/ignores"
)

// updateEvent is used to store information about an update/create event.
type updateEvent struct {
	Path   string
	Rel    string
	Status string
}

// Event describes a file event and the associated container.
type Event struct {
	Container *schemas.Container `json:"container"`
	Status    string             `json:"status"`
	Paths     []string           `json:"paths"`
}

// WatchError wraps an error to identify the container origin.
type WatchError struct {
	Container *schemas.Container `json:"container"`
	Err       error              `json:"error"`
}

func (w *WatchError) Error() string {
	return w.Err.Error()
}

// Watcher syncs file changes for a container to it's remote address.
type Watcher struct {
	Container *schemas.Container
	mutex     sync.Mutex
	done      chan struct{}
	isDone    bool
}

// NewWatcher creates a watcher.
func NewWatcher(container *schemas.Container) *Watcher {
	var mutex sync.Mutex

	return &Watcher{
		Container: container,
		mutex:     mutex,
		done:      make(chan struct{}),
	}
}

// Start syncs file changes and uploads to the applications remote address.
func (watcher *Watcher) Start(evChan chan *Event, errChan chan error) {
	var found []string
	stats := make(map[string]os.FileInfo)
	updates := make([]*updateEvent, 0)
	local := watcher.Container.LocalPath

	// If previously called Close reset the state.
	watcher.mutex.Lock()
	if watcher.isDone {
		watcher.isDone = false
		watcher.done = make(chan struct{})
	}
	watcher.mutex.Unlock()

	ignoreList, err := ignores.Get(filepath.Join(local, config.IgnorePath))
	if err != nil {
		errChan <- watcher.wrapErr(err)
		ignoreList = make([]string, 0)
	}

	// Get initial stats.
	err = filepath.Walk(local, func(path string, info os.FileInfo, err error) error {
		if err != nil || local == path {
			if os.IsNotExist(err) {
				err = nil
			}

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
		if err != nil && !os.IsNotExist(err) {
			errChan <- watcher.wrapErr(err)
			return nil
		}
		if err != nil || local == path {
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
			status = delancey.UpdateStatus
		} else if !ok {
			status = delancey.CreateStatus
		}
		stats[path] = info
		found = append(found, path)

		// Ignore if no change has occured.
		if status == "" {
			return nil
		}

		updates = append(updates, &updateEvent{Path: path, Rel: rel, Status: status})
		return nil
	}

	// Manages deletes.
	checkDeletes := func() {
		delList := make(sort.StringSlice, 0)
		delStats := make(map[string]os.FileInfo)

		// Get a list of paths to delete.
		for path, stat := range stats {
			skip := false
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
			delList = append(delList, path)
			delStats[path] = stat
		}

		sort.Sort(delList)
		rootList := make(map[string]os.FileInfo)

		// Do the deletes.
		for _, path := range delList {
			// Check if the path or a parent dir has already done the event.
			skip := false
			for alreadyDone, stat := range rootList {
				if path == alreadyDone ||
					(stat.IsDir() && strings.Contains(path, alreadyDone+string(filepath.Separator))) {
					skip = true
					break
				}
			}
			if skip {
				continue
			}
			rootList[path] = delStats[path]

			rel, err := filepath.Rel(local, path)
			if err != nil {
				errChan <- watcher.wrapErr(err)
				continue
			}

			err = watcher.Update(rel, delancey.DeleteStatus)
			if err != nil {
				errChan <- watcher.wrapErr(err)
				continue
			}

			evChan <- &Event{Container: watcher.Container, Status: delancey.DeleteStatus, Paths: []string{rel}}
		}
	}

	// Removes a temp path from the state, so false deletes aren't triggered.
	removeTemp := func(path string) {
		delete(stats, path)
		found = util.RemoveFromSlice(found, path)
	}

	// Standard update, does them one at a time.
	standardUpdate := func() {
		for _, ev := range updates {
			err = watcher.Update(ev.Rel, ev.Status)
			if err != nil {
				if os.IsNotExist(err) {
					removeTemp(ev.Path)
					continue
				}

				errChan <- watcher.wrapErr(err)
				continue
			}

			evChan <- &Event{Container: watcher.Container, Status: ev.Status, Paths: []string{ev.Rel}}
		}
	}

	// Batch update, sends all of them in a single .tar.gz upload.
	batchUpdate := func() {
		batchChan := make(chan error)
		pathList := make([]string, 0, len(updates))
		paths := make(map[string]string, len(updates))

		for _, ev := range updates {
			pathList = append(pathList, ev.Path)
			paths[ev.Path] = ev.Rel
		}

		go func() {
			for err := range batchChan {
				berr, ok := err.(*delancey.BatchError)
				if ok && os.IsNotExist(berr.Err) {
					removeTemp(berr.Path)
					continue
				}

				errChan <- watcher.wrapErr(err)
			}
		}()

		evChan <- &Event{Container: watcher.Container, Status: delancey.BatchStartStatus, Paths: pathList}
		err := delancey.BatchUpdate(watcher.Container, paths, batchChan)
		if err != nil {
			errChan <- watcher.wrapErr(err)
			return
		}

		evChan <- &Event{Container: watcher.Container, Status: delancey.BatchFinishStatus, Paths: pathList}
	}

	for {
		// Check if we're done.
		select {
		case <-watcher.done:
			return
		default:
		}

		ignoreList, err = ignores.Get(filepath.Join(local, config.IgnorePath))
		if err != nil {
			errChan <- watcher.wrapErr(err)
			ignoreList = make([]string, 0)
		}

		err = filepath.Walk(local, walker)
		if err != nil {
			errChan <- watcher.wrapErr(err)
		}
		isBatchJob := len(updates) > 16

		// Do the create/update uploads.
		if isBatchJob {
			batchUpdate()
		} else {
			standardUpdate()
		}

		checkDeletes()
		updates = make([]*updateEvent, 0)
		found = make([]string, 0)
		<-time.After(500 * time.Millisecond)
	}
}

// Upload compresses and uploads the contents to the applications remote address.
func (watcher *Watcher) Upload() error {
	var (
		err error
	)
	local := watcher.Container.LocalPath
	i := 0

	ignoreList, err := ignores.Get(filepath.Join(local, config.IgnorePath))
	if err != nil {
		return watcher.wrapErr(err)
	}

	// Tar up the path and write to a type supporting seeking.
	upload, err := tar.Tar(local, ignoreList)
	if err != nil {
		return watcher.wrapErr(err)
	}
	var buf bytes.Buffer
	_, err = io.Copy(&buf, upload)
	if err != nil {
		return watcher.wrapErr(err)
	}
	uploadContents := bytes.NewReader(buf.Bytes())

	// Attempt to upload, ensuring the upload is at the beginning of the file.
	for i < 1000 {
		_, err = uploadContents.Seek(0, os.SEEK_SET)
		if err != nil {
			return watcher.wrapErr(err)
		}

		err = delancey.Upload(watcher.Container, uploadContents)
		if err == nil {
			return nil
		}

		i++
		<-time.After(time.Millisecond * 50)
	}

	return watcher.wrapErr(err)
}

// Update updates a path to the containers remote address.
func (watcher *Watcher) Update(name, status string) error {
	path := filepath.Join(watcher.Container.LocalPath, name)

	err := delancey.Update(watcher.Container, path, name, status)
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

		return delancey.Update(watcher.Container, path, name, status)
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

	return nil
}

// wrapErr wraps an error with the application it occurred for.
func (watcher *Watcher) wrapErr(err error) error {
	if err == nil {
		return nil
	}

	return &WatchError{Container: watcher.Container, Err: err}
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

// GetWatcher gets a watcher for a specific container.
func (syncer *Syncer) GetWatcher(container *schemas.Container) (*Watcher, bool) {
	for _, watcher := range syncer.Watchers {
		if watcher != nil && watcher.Container.ID == container.ID {
			return watcher, false
		}
	}

	return nil, true
}

// Watch starts watching the given container syncing changes.
func (syncer *Syncer) Watch(container *schemas.Container) {
	watcher := NewWatcher(container)
	syncer.Watchers = append(syncer.Watchers, watcher)

	// Do the actual event management, and the inital upload.
	go func() {
		syncer.Event <- &Event{Container: watcher.Container, Status: delancey.UploadStartStatus}
		err := watcher.Upload()
		if err != nil {
			syncer.Error <- err
			return
		}
		syncer.Event <- &Event{Container: watcher.Container, Status: delancey.UploadFinishStatus}

		watcher.Start(syncer.Event, syncer.Error)
	}()
}

// Remove removes a containers syncer.
func (syncer *Syncer) Remove(container *schemas.Container) error {
	for idx, watcher := range syncer.Watchers {
		if watcher != nil && watcher.Container.ID == container.ID {
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
