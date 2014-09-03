// Copyright 2014 Bowery, Inc.
package main

import (
	"errors"
	"fmt"
	"log"

	"github.com/Bowery/gopackages/schemas"
)

type ApplicationManager struct {
	Applications map[string]*schemas.Application
	Syncer       *Syncer
}

func NewApplicationManager() *ApplicationManager {
	return &ApplicationManager{
		Applications: make(map[string]*schemas.Application),
		Syncer:       NewSyncer(),
	}
}

func (am *ApplicationManager) load(token string) error {
	apps, err := GetApplications(token)
	if err != nil {
		return err
	}

	log.Println(apps)

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

	am.Syncer.Watch(app)
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

func (am *ApplicationManager) UpdateByID(id string, app *schemas.Application) error {
	_, ok := am.Applications[id]
	if !ok {
		return errors.New("invalid app id")
	}

	am.Applications[app.ID] = app
	return nil
}

func (am *ApplicationManager) Remove(id string) (*schemas.Application, error) {
	app, ok := am.Applications[id]
	if !ok {
		return nil, fmt.Errorf("no app with id %s exists.", id)
	}

	err := DelanceyRemove(app)
	if err != nil {
		return nil, err
	}

	err = am.Syncer.Remove(app)
	if err != nil {
		return nil, err
	}

	delete(am.Applications, id)
	return app, nil
}

func (am *ApplicationManager) Close() error {
	return am.Syncer.Close()
}
