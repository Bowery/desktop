// Copyright 2014 Bowery, Inc.
package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/Bowery/gopackages/config"
	"github.com/Bowery/gopackages/requests"
	"github.com/Bowery/gopackages/rollbar"
	"github.com/Bowery/gopackages/schemas"
)

func init() {
	rollbarC = rollbar.NewClient("", "testing")
	applicationManager = NewApplicationManager()
	defer applicationManager.Close()
}

var (
	dir, _ = os.Getwd()
)

func TestCreateApplicationHandlerSuccessful(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(createApplicationHandler))
	defer server.Close()

	kenmareServer := httptest.NewServer(http.HandlerFunc(kenmareCreateApplicationHandlerSuccessful))
	defer kenmareServer.Close()
	config.KenmareAddr = kenmareServer.URL

	reqBody := applicationReq{
		AMI:          "ami-722ff51a",
		EnvID:        "some-id",
		Token:        "some-token",
		InstanceType: "m1.small",
		AWSAccessKey: "some-key",
		AWSSecretKey: "some-secret",
		Ports:        "5000",
		Name:         "my-hot-new-app",
		Start:        "start the app",
		Build:        "build the app",
		LocalPath:    dir,
		RemotePath:   "/home/ubuntu/app",
	}

	var data bytes.Buffer
	encoder := json.NewEncoder(&data)
	err := encoder.Encode(reqBody)
	if err != nil {
		t.Error(err)
	}

	res, err := http.Post(server.URL, "application/json", &data)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		t.Error("request failed with non 200 status code.")
	}
}

func kenmareCreateApplicationHandlerSuccessful(rw http.ResponseWriter, req *http.Request) {
	r.JSON(rw, http.StatusOK, map[string]interface{}{
		"status": requests.STATUS_SUCCESS,
		"application": schemas.Application{
			ID:        "some-id",
			LocalPath: dir,
		},
	})
}

func TestCreateApplicationHandlerMissingFields(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(createApplicationHandler))
	defer server.Close()

	reqBody := applicationReq{
		AMI: "ami-722ff51a",
	}

	var data bytes.Buffer
	encoder := json.NewEncoder(&data)
	err := encoder.Encode(reqBody)
	if err != nil {
		t.Error(err)
	}

	res, err := http.Post(server.URL, "application/json", &data)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusBadRequest {
		t.Error("request did not fail as expected.")
	}
}
