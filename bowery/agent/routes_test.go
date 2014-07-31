// Copyright 2014 Bowery, Inc.
package main

import (
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetHealthz(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(HealthzHandler))
	defer server.Close()

	res, err := http.Get(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()

	if res.StatusCode != 200 {
		t.Error("Status Code of Healthz was not 200.")
	}

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		t.Fatal("Unable to read response body.")
	}

	if string(body) != "ok" {
		t.Error("Healthz body was not ok.")
	}
}
