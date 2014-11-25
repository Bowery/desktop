// Copyright 2014 Bowery, Inc.
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

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

type environmentRes struct {
	*Res
	Environment *schemas.Environment `json:"environment"`
}

type environmentsRes struct {
	*Res
	Environments []*schemas.Environment `json:"environments"`
}

type createEventReq struct {
	Type  string `json:"type"`
	Body  string `json:"body"`
	EnvID string `json:"envID"`
}

func CreateApplication(reqBody *applicationReq) (*schemas.Application, error) {
	var data bytes.Buffer
	encoder := json.NewEncoder(&data)
	err := encoder.Encode(reqBody)
	if err != nil {
		return nil, err
	}

	addr := fmt.Sprintf("%s/applications", config.KenmareAddr)
	res, err := http.Post(addr, "application/json", &data)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	// Parse response.
	var resBody applicationRes
	decoder := json.NewDecoder(res.Body)
	err = decoder.Decode(&resBody)
	if err != nil {
		return nil, err
	}

	if resBody.Status == requests.StatusFailed {
		return nil, resBody
	}

	return resBody.Application, nil
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

	if appsRes.Status != requests.StatusFound {
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

	if appRes.Status != requests.StatusFound {
		return nil, appRes
	}

	return appRes.Application, nil
}

func CreateEvent(app *schemas.Application, cmd string) error {
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

func SearchEnvironments(query string) ([]*schemas.Environment, error) {
	addr := fmt.Sprintf("%s/environments?query=%s", config.KenmareAddr, query)
	res, err := http.Get(addr)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	searchRes := new(environmentsRes)
	decoder := json.NewDecoder(res.Body)
	err = decoder.Decode(searchRes)
	if err != nil {
		return nil, err
	}

	if searchRes.Status != requests.StatusFound {
		return nil, searchRes
	}

	return searchRes.Environments, nil
}

func GetEnvironment(id string) (*schemas.Environment, error) {
	addr := fmt.Sprintf("%s/environments/%s", config.KenmareAddr, id)
	res, err := http.Get(addr)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	envRes := new(environmentRes)
	decoder := json.NewDecoder(res.Body)
	err = decoder.Decode(envRes)
	if err != nil {
		return nil, err
	}

	if envRes.Status != requests.StatusFound {
		return nil, envRes
	}

	return envRes.Environment, nil
}

type updateEnvReq struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Token       string `json:"token"`
}

func UpdateEnvironment(env *schemas.Environment, token string) (*schemas.Environment, error) {
	req := &updateEnvReq{
		Name:        env.Name,
		Description: env.Description,
		Token:       token,
	}

	var body bytes.Buffer
	encoder := json.NewEncoder(&body)
	err := encoder.Encode(req)
	if err != nil {
		return nil, err
	}

	addr := fmt.Sprintf("%s/environments/%s", config.KenmareAddr, env.ID)
	request, err := http.NewRequest("PUT", addr, &body)
	if err != nil {
		return nil, err
	}

	request.Header.Set("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(request)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	var resBody environmentRes
	decoder := json.NewDecoder(res.Body)
	err = decoder.Decode(&resBody)
	if err != nil {
		return nil, err
	}

	if resBody.Status != requests.StatusSuccess {
		return nil, resBody
	}

	return resBody.Environment, nil
}

func ValidateKeys(access, secret string) error {
	payload := make(url.Values)
	payload.Add("aws_access_key", access)
	payload.Add("aws_secret_key", secret)

	endpoint := "auth/validate-keys"
	queryParameters := fmt.Sprintf("?%s", payload.Encode())
	addr := fmt.Sprintf("%s/%s%s", config.KenmareAddr, endpoint, queryParameters)

	res, err := http.Get(addr)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	resBody := new(Res)
	decoder := json.NewDecoder(res.Body)
	err = decoder.Decode(resBody)
	if err != nil {
		return err
	}

	if resBody.Status != requests.StatusSuccess {
		return resBody
	}

	return nil
}
