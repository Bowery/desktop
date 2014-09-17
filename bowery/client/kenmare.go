// Copyright 2014 Bowery, Inc.
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/Bowery/gopackages/config"
	"github.com/Bowery/gopackages/requests"
	"github.com/Bowery/gopackages/schemas"
)

type applicationRes struct {
	*Res
	Application *schemas.Application `json:"application"`
}

type applicationsRes struct {
	*Res
	Applications []*schemas.Application `json:"applications"`
}

type createEventReq struct {
	Type  string `json:"type"`
	Body  string `json:"body"`
	EnvID string `json:"envID"`
}

func GetApplications(token string) ([]*schemas.Application, error) {
	addr := fmt.Sprintf("%s/applications?token=%s", config.KenmareAddr, token)
	res, err := http.Get(addr)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	appsRes := new(applicationsRes)
	decoder := json.NewDecoder(res.Body)
	err = decoder.Decode(appsRes)
	if err != nil {
		return nil, err
	}

	if appsRes.Status != requests.STATUS_FOUND {
		return nil, appsRes
	}

	return appsRes.Applications, nil
}

func GetApplication(id string) (*schemas.Application, error) {
	addr := fmt.Sprintf("%s/applications/%s", config.KenmareAddr, id)
	res, err := http.Get(addr)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	appRes := new(applicationRes)
	decoder := json.NewDecoder(res.Body)
	err = decoder.Decode(appRes)
	if err != nil {
		return nil, err
	}

	if appRes.Status != requests.STATUS_FOUND {
		return nil, appRes
	}

	return appRes.Application, nil
}

func KenmareCreateEvent(app *schemas.Application, cmd string) error {
	req := &createEventReq{
		Type:  "command",
		EnvID: app.EnvID,
		Body:  cmd,
	}

	var body bytes.Buffer
	encoder := json.NewEncoder(&body)
	err := encoder.Encode(req)
	if err != nil {
		return err
	}

	url := fmt.Sprintf("%s/events", config.KenmareAddr)
	res, err := http.Post(url, "application/json", &body)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	// Decode json response.
	createRes := new(Res)
	decoder := json.NewDecoder(res.Body)
	err = decoder.Decode(createRes)
	if err != nil {
		return err
	}

	if createRes.Status == "success" {
		return nil
	}

	return createRes
}
