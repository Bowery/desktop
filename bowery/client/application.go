// Copyright 2014 Bowery, Inc.
package main

import (
	"errors"
	"fmt"
	"log"
	"net"
	"time"

	"github.com/Bowery/gopackages/schemas"
)

type ApplicationManager struct {
	Applications  map[string]*schemas.Application
	Syncer        *Syncer
	StreamManager *StreamManager
}

func NewApplicationManager() *ApplicationManager {
	return &ApplicationManager{
		Applications:  make(map[string]*schemas.Application),
		Syncer:        NewSyncer(),
		StreamManager: NewStreamManager(),
	}
}

func (am *ApplicationManager) load(token string) error {
	apps, err := GetApplications(token)
	if err != nil {
		return err
	}

	for _, app := range apps {
		err = am.Add(app)
		if err != nil {
			return err
		}
	}

	return nil
}

func (am *ApplicationManager) Add(app *schemas.Application) error {
	if app.ID == "" {
		return errors.New("application must have a valid id.")
	}

	// Initiate file syncing and stream connection
	// once the agent becomes available.
	go func() {
		for app != nil && !app.IsSyncAvailable && app.Location != "" {
			<-time.After(1 * time.Second)
			log.Println("checking agent...")
			err := DelanceyCheck(net.JoinHostPort(app.Location, "32056"))
			if err == nil {
				app.IsSyncAvailable = true
				app.Status = "running"
				msg := map[string]interface{}{
					"appID":   app.ID,
					"type":    "status",
					"message": app.Status,
				}
				ssePool.messages <- msg
			}
		}

		log.Println("agent available!")

		am.Syncer.Watch(app)
		am.StreamManager.Connect(app)
	}()

	am.Applications[app.ID] = app
	return nil
}

func (am *ApplicationManager) GetAll(token string) ([]*schemas.Application, error) {
	// Only fetch applications if this is the first load.
	// todo(steve): do this better.
	if len(am.Applications) == 0 {
		if err := am.load(token); err != nil {
			return nil, err
		}
	}

	apps := []*schemas.Application{}
	for _, a := range am.Applications {
		apps = append(apps, a)
	}

	return apps, nil
}

func (am *ApplicationManager) GetByID(id string) (*schemas.Application, error) {
	app, ok := am.Applications[id]
	if !ok {
		return nil, fmt.Errorf("no app with id %s exists.", id)
	}

	return app, nil
}

func (am *ApplicationManager) UpdateByID(id string, changes *schemas.Application) (*schemas.Application, error) {
	app, ok := am.Applications[id]
	if !ok {
		return nil, errors.New("invalid app id")
	}

	if changes.Name != "" {
		app.Name = changes.Name
	}
	if changes.Location != "" {
		app.Location = changes.Location
	}
	if changes.RemotePath != "" {
		app.RemotePath = changes.RemotePath
	}
	if changes.LocalPath != "" {
		app.LocalPath = changes.LocalPath
	}

	// Empty commands are ok.
	app.Start = changes.Start
	app.Build = changes.Build

	// Reset the syncer so a upload is done.
	am.Syncer.Remove(app)
	am.Syncer.Watch(app)

	// Reset the log manager.
	am.StreamManager.Remove(app)
	am.StreamManager.Connect(app)
	return app, nil
}

func (am *ApplicationManager) RemoveByID(id string) (*schemas.Application, error) {
	app, ok := am.Applications[id]
	if !ok {
		return nil, fmt.Errorf("no app with id %s exists.", id)
	}

	err := am.Syncer.Remove(app)
	if err != nil {
		return nil, err
	}

	err = am.StreamManager.Remove(app)
	if err != nil {
		return nil, err
	}

	delete(am.Applications, id)
	return app, nil
}

func (am *ApplicationManager) Close() error {
	return am.Syncer.Close()
}

func (am *ApplicationManager) Empty() {
	for _, app := range am.Applications {
		am.Syncer.Remove(app)
		am.StreamManager.Remove(app)
	}

	am.Applications = make(map[string]*schemas.Application)
}
