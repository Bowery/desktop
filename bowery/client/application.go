// Copyright 2014 Bowery, Inc.
package main

import (
	"errors"
	"fmt"
	"log"
	"net"
	"sort"
	"strings"
	"time"

	"github.com/Bowery/gopackages/config"
	"github.com/Bowery/gopackages/schemas"
	"github.com/Bowery/gopackages/sys"
	"github.com/jeffchao/backoff"
)

// Note: Order is lower the index the higher the priority.
var (
	portsPriority = config.SuggestedPorts[:len(config.SuggestedPorts)-1]
	listeners     = make(map[int]*sys.Listener, len(portsPriority))
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
		status := app.Status

		// Check the application status every 5 seconds
		// via Kenmare. If the app status changes to
		// running proceed.
		exponential := backoff.Exponential()
		exponential.MaxRetries = 200

		for app != nil && status != "running" {
			if !exponential.Next() {
				return
			}
			<-time.After(exponential.Delay)
			application, err := GetApplication(app.ID)
			if err == nil {
				log.Println("provisioning status: " + application.Status)
				status = application.Status

				// The instance is running but still display as provisioning because
				// the app hasn't been created on the agent yet.
				if status == "running" {
					app.Status = "provisioning"
				} else {
					app.Status = status
				}
				app.Location = application.Location
				msg := map[string]interface{}{
					"appID":   app.ID,
					"type":    "status",
					"message": app,
				}
				ssePool.messages <- msg
				if status == "running" {
					break
				}
			} else if strings.Contains(err.Error(), "Not Found") {
				// If the application can't be found then it's been deleted.
				return
			}
		}

		exponential = backoff.Exponential()
		exponential.MaxRetries = 200

		// Ping the agent to verify it's healthy. Once a healthy
		// response is returned, update the database.
		for app != nil && !app.IsSyncAvailable {
			if !exponential.Next() {
				return
			}
			<-time.After(exponential.Delay)
			err := DelanceyCheck(net.JoinHostPort(app.Location, "32056"))
			if err != nil {
				continue
			}

			app.IsSyncAvailable = true
			app.Status = "running"
			SetAppPort(app)
			msg := map[string]interface{}{
				"appID":   app.ID,
				"type":    "status",
				"message": app,
			}
			ssePool.messages <- msg
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
	}

	for _, a := range am.Applications {
		appsArray = append(appsArray, a)
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

	if changes == nil {
		return app, nil
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

// SetAppPort will detect the port in use and set it for the app.
func SetAppPort(app *schemas.Application) error {
	if app.Status != "running" || app.Location == "" {
		return nil
	}

	appSpecific, generic, err := DelanceyNetwork(app)
	if err != nil {
		return err
	}

	// Clear listeners.
	for _, port := range portsPriority {
		listeners[port] = nil
	}

	// Get generic listeners that are in priority list.
	for _, listener := range generic {
		_, ok := listeners[listener.Port]

		if ok {
			listeners[listener.Port] = listener
		}
	}

	// Get apps listeners that are in priority list.
	for _, listener := range appSpecific {
		_, ok := listeners[listener.Port]

		if ok {
			listeners[listener.Port] = listener
		}
	}

	// Get the listener with the greatest priority, prefer lower index ports.
	var listener *sys.Listener
	for _, port := range portsPriority {
		list := listeners[port]

		if list != nil {
			listener = list
			break
		}
	}

	// Fallback to first item found.
	if listener == nil {
		if len(appSpecific) > 0 {
			listener = appSpecific[0]
		} else if len(generic) > 0 {
			listener = generic[0]
		}
	}

	if listener != nil {
		app.PortInUse = listener.Port
	}

	return nil
}
