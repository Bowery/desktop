// Copyright 2014 Bowery, Inc.
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/Bowery/gopackages/config"
	"github.com/Bowery/gopackages/requests"
	"github.com/Bowery/gopackages/schemas"
)

type applicationRes struct {
	Status      string               `json:"status"`
	Error       string               `json:"error"`
	Application *schemas.Application `json:"application"`
}

type applicationsRes struct {
	Status       string                 `json:"status"`
	Error        string                 `json:"error"`
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
		return nil, errors.New(appsRes.Error)
	}

	return appsRes.Applications, nil
}
