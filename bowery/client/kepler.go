// Copyright 2014 Bowery, Inc.
package main

import (
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
