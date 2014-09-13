// Copyright 2014 Bowery, Inc.
package main

import (
	"testing"

	"github.com/Bowery/gopackages/schemas"
)

var (
	testApplicationManager *ApplicationManager
	testApplication        = &schemas.Application{
		ID:          "some-id",
		Name:        "some-name",
		DeveloperID: "some-id",
		EnvID:       "some-id",
		Location:    "0.0.0.0",
		RemotePath:  "/home/ubuntu/myproject",
		LocalPath:   "/my/desktop/myproject",
		Status:      "running",
		Start:       "echo start",
		Build:       "echo build",
	}
)

func init() {
	testApplicationManager = NewApplicationManager()
}

func TestAdd(t *testing.T) {
	err := testApplicationManager.Add(testApplication)
	if err != nil {
		t.Error(err)
	}
}

func TestGetByID(t *testing.T) {
	app, err := testApplicationManager.GetByID("some-id")
	if err != nil {
		t.Error(err)
	}

	if app.ID != testApplication.ID {
		t.Fatal("incorrect application selected.")
	}
}

func TestUpdateByIDValidField(t *testing.T) {
	changes := &schemas.Application{
		Name: "new-name",
	}

	app, err := testApplicationManager.UpdateByID(testApplication.ID, changes)
	if err != nil {
		t.Error(err)
	}

	if app.Name != "new-name" {
		t.Fatal("failed to udpate application.")
	}
}

func TestUpdateByIDInvalidField(t *testing.T) {
	changes := &schemas.Application{
		Status: "provisioning",
	}

	app, err := testApplicationManager.UpdateByID(testApplication.ID, changes)
	if err != nil {
		t.Fatal(err)
	}

	if app.Status == "provisioning" {
		t.Fatal("updated invalid field.")
	}
}
