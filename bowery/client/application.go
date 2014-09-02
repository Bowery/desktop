// Copyright 2014 Bowery, Inc.
package main

import (
	"errors"
	"fmt"

	"github.com/Bowery/gopackages/schemas"
)

type ApplicationManager struct {
	applications map[string]*schemas.Application
}

func NewApplicationManager() *ApplicationManager {
	am := &ApplicationManager{
		applications: map[string]*schemas.Application{},
	}

	return am
}

func (am *applicationManager) load() {

}

func (am *ApplicationManager) Add(app *schemas.Application) error {
	if app.ID == "" {
		return errors.New("application must have a valid id.")
	}

	am.applications[app.ID] = app

	return nil
}

func (am *ApplicationManager) GetAll() ([]*schemas.Application, error) {
	apps := make([]*schemas.Application, len(am.applications))
	for _, a := range am.applications {
		apps = append(apps, a)
	}
	return apps, nil
}

func (am *ApplicationManager) GetByID(id string) (*schemas.Application, error) {
	app, ok := am.applications[id]
	if !ok {
		return nil, fmt.Errorf("no app with id %s exists.", id)
	}

	return app, nil
}
