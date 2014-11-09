// Copyright 2014 Bowery, Inc.
package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Bowery/gopackages/config"
	"github.com/Bowery/gopackages/requests"
	"github.com/Bowery/gopackages/schemas"
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
		t.Fatal("request did not succeed as expected")
	}
}

func kenmareValidateKeysHandlerSuccessful(rw http.ResponseWriter, req *http.Request) {
	r.JSON(rw, http.StatusOK, map[string]interface{}{
		"status": requests.STATUS_SUCCESS,
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
	r.JSON(rw, http.StatusOK, map[string]interface{}{
		"status": requests.STATUS_FAILED,
		"error":  "invalid keys",
	})
}

func TestShareEnvironmentSuccess(t *testing.T) {
	kenmareServer := httptest.NewServer(http.HandlerFunc(kenmareShareEnvironmentHandlerSuccessful))
	defer kenmareServer.Close()
	config.KenmareAddr = kenmareServer.URL

	_, err := ShareEnvironment("some-env", "some-token", "some-user@bowery.io")
	if err != nil {
		t.Error("request did not succeed as expected")
	}
}

func kenmareShareEnvironmentHandlerSuccessful(rw http.ResponseWriter, req *http.Request) {
	r.JSON(rw, http.StatusOK, map[string]interface{}{
		"status":      requests.STATUS_SUCCESS,
		"environment": schemas.Environment{},
	})
}

func TestShareEnvironmentMissingField(t *testing.T) {
	_, err := ShareEnvironment("some-env", "", "some-user@bowery.io")
	if err == nil || err.Error() != "envID, token, and email required." {
		t.Error("request did not fail as expected")
	}
}

func TestShareEnvironmentInvalidEmail(t *testing.T) {
	_, err := ShareEnvironment("some-env", "some-token", "some-email#bowery.io")
	if err == nil || err.Error() != "invalid email" {
		t.Error("request did not fail as expected")
	}
}
