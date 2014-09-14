// Copyright 2014 Bowery, Inc.
package main

import (
	"errors"
	"fmt"
	"log"
	"net"
	"sort"
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

	// Check the status of the application on Kenmare.
	go func() {
		// Check the application status every 5 seconds
		// via Kenmare. If the app status changes to
		// running proceed.
		for app != nil && app.Status != "running" {
			application, err := GetApplication(app.ID)
			if err != nil {
				continue
			}
			log.Println("provisioning status: " + application.Status)
			app.Status = application.Status
			app.Location = application.Location
			msg := map[string]interface{}{
				"appID":   app.ID,
				"type":    "status",
				"message": app,
			}
			ssePool.messages <- msg
			<-time.After(5 * time.Second)
		}

		// Ping the agent to verify it's healthy. Once a healthy
		// response is returned, update the database.
		for app != nil && !app.IsSyncAvailable {
			err := DelanceyCheck(net.JoinHostPort(app.Location, "32056"))
			if err == nil {
				app.IsSyncAvailable = true
				app.Status = "running"
				msg := map[string]interface{}{
					"appID":   app.ID,
					"type":    "status",
					"message": app,
				}
				ssePool.messages <- msg
				break
			}
			<-time.After(5 * time.Second)
		}

		if app != nil {
			// Update application.
			application, _ := GetApplication(app.ID)
			app, _ = am.UpdateByID(app.ID, application)
		}
	}()

	am.Applications[app.ID] = app
	return nil
}

// byCreatedAt implements the Sort interface for
// a slice of applications.
type byCreatedAt []*schemas.Application

func (v byCreatedAt) Len() int           { return len(v) }
func (v byCreatedAt) Swap(i, j int)      { v[i], v[j] = v[j], v[i] }
func (v byCreatedAt) Less(i, j int) bool { return v[i].CreatedAt.Unix() < v[j].CreatedAt.Unix() }

func (am *ApplicationManager) GetAll(token string) ([]*schemas.Application, error) {
	appsArray := []*schemas.Application{}
	if len(am.Applications) == 0 {
		if err := am.load(token); err != nil {
			return nil, err
		}

		for _, a := range am.Applications {
			appsArray = append(appsArray, a)
		}
	} else {
		apps, err := GetApplications(token)
		if err != nil {
			return nil, err
		}

		for _, a := range apps {
			appsArray = append(appsArray, a)
		}
	}

	sort.Sort(sort.Reverse(byCreatedAt(appsArray)))
	return appsArray, nil
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

	// If everything is empty then ignore it
	if changes.Name == "" &&
		changes.Location == "" &&
		changes.RemotePath == "" &&
		changes.LocalPath == "" &&
		changes.Start == "" &&
		changes.Status == "" &&
		changes.Build == "" {
		return nil, errors.New("invalid requests")
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

	// Reset the syncer so an upload is done.
	if app.Location != "" && app.IsSyncAvailable {
		if err := am.Syncer.Remove(app); err != nil {
			log.Println("StreamManager.Remove Failed in UpdateByID", err)
		}
		am.Syncer.Watch(app)

		// Reset the log manager.
		am.StreamManager.Remove(app)
		am.StreamManager.Connect(app)
	}
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
	err := am.StreamManager.Close()
	if err != nil {
		return err
	}

	return am.Syncer.Close()
}

func (am *ApplicationManager) Empty() {
	for _, app := range am.Applications {
		am.Syncer.Remove(app)
		am.StreamManager.Remove(app)
	}

	am.Applications = make(map[string]*schemas.Application)
}
