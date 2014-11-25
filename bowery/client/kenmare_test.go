// Copyright 2014 Bowery, Inc.
package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Bowery/gopackages/config"
	"github.com/Bowery/gopackages/requests"
)

var (
	testAccessKey = "testAccessKey"
	testSecretKey = "testSecretKey"
)

func TestValidateKeysSuccess(t *testing.T) {
	kenmareServer := httptest.NewServer(http.HandlerFunc(kenmareValidateKeysHandlerSuccessful))
	defer kenmareServer.Close()
	config.KenmareAddr = kenmareServer.URL

	err := ValidateKeys(testAccessKey, testSecretKey)
	if err != nil {
		t.Fatal("request did not success as expected")
	}
}

func kenmareValidateKeysHandlerSuccessful(rw http.ResponseWriter, req *http.Request) {
	renderer.JSON(rw, http.StatusOK, map[string]interface{}{
		"status": requests.StatusSuccess,
	})
}

func TestValidateKeysFailure(t *testing.T) {
	kenmareServer := httptest.NewServer(http.HandlerFunc(kenmareValidateKeysHandlerBadKeys))
	defer kenmareServer.Close()
	config.KenmareAddr = kenmareServer.URL

	err := ValidateKeys(testAccessKey, testSecretKey)
	if err == nil {
		t.Fatal("request did not fail as expected")
	}
}

func kenmareValidateKeysHandlerBadKeys(rw http.ResponseWriter, req *http.Request) {
	renderer.JSON(rw, http.StatusOK, map[string]interface{}{
		"status": requests.StatusFailed,
		"error":  "invalid keys",
	})
}
