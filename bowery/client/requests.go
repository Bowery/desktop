// Copyright 2014 Bowery, Inc.
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/Bowery/gopackages/config"
	"github.com/Bowery/gopackages/requests"
	"github.com/Bowery/gopackages/schemas"
	"github.com/gorilla/mux"
	"github.com/unrolled/render"
)

type Route struct {
	Method  string
	Path    string
	Handler http.HandlerFunc
}

type SlashHandler struct {
	Handler http.Handler
}

func (sh *SlashHandler) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	if req.URL.Path != "/" {
		req.URL.Path = strings.TrimRight(req.URL.Path, "/")
		req.RequestURI = req.URL.RequestURI()
	}

	sh.Handler.ServeHTTP(rw, req)
}

var Routes = []*Route{
	&Route{"POST", "/applications", createApplicationHandler},
	&Route{"GET", "/applications", getApplicationsHandler},
	&Route{"POST", "/applications/{id}", updateApplicationHandler},
	&Route{"GET", "/applications/{id}", getApplicationHandler},
	&Route{"DELETE", "/applications/{id}", removeApplicationHandler},
}

var r = render.New(render.Options{
	IndentJSON:    true,
	IsDevelopment: true,
})

type applicationReq struct {
	AMI          string `json:"ami"`
	EnvID        string `json:"envID"`
	Token        string `json:"token"`
	InstanceType string `json:"instance_type"`
	AWSAccessKey string `json:"aws_access_key"`
	AWSSecretKey string `json:"aws_secret_key"`
	Ports        string `json:"ports"`
	Name         string `json:"name"`
	Start        string `json:"start"`
	Build        string `json:"build"`
	LocalPath    string `json:"localPath"`
	RemotePath   string `json:"remotePath"`
}

// Res is a generic response with status and an error message.
type Res struct {
	Status string `json:"status"`
	Err    string `json:"error"`
}

func (res *Res) Error() string {
	return res.Err
}

// createApplicationHandler creates a new Application which includes:
// persisting it in a db and creating a new env via Kenmare, sending
// application files and commands to the remote agent, and initiating
// file watching.
func createApplicationHandler(rw http.ResponseWriter, req *http.Request) {
	// Parse request.
	var reqBody applicationReq
	decoder := json.NewDecoder(req.Body)
	err := decoder.Decode(&reqBody)
	if err != nil {
		r.JSON(rw, http.StatusBadRequest, map[string]string{
			"status": requests.STATUS_FAILED,
			"error":  err.Error(),
		})
		return
	}

	if reqBody.AMI == "" {
		reqBody.AMI = "ami-722ff51a"
	}

	// Validate request.
	if reqBody.InstanceType == "" || reqBody.AWSAccessKey == "" ||
		reqBody.AWSSecretKey == "" || reqBody.Token == "" {
		r.JSON(rw, http.StatusBadRequest, map[string]string{
			"status": requests.STATUS_FAILED,
			"error":  "missing fields",
		})
		return
	}

	// Encode request.
	var data bytes.Buffer
	encoder := json.NewEncoder(&data)
	err = encoder.Encode(reqBody)
	if err != nil {
		r.JSON(rw, http.StatusInternalServerError, map[string]string{
			"status": requests.STATUS_FAILED,
			"error":  err.Error(),
		})
		return
	}

	// Send request.
	addr := fmt.Sprintf("%s/applications", config.KenmareAddr)
	res, err := http.Post(addr, "application/json", &data)
	if err != nil {
		r.JSON(rw, http.StatusInternalServerError, map[string]string{
			"status": requests.STATUS_FAILED,
			"error":  err.Error(),
		})
		return
	}
	defer res.Body.Close()

	// Parse response.
	var resBody applicationRes
	decoder = json.NewDecoder(res.Body)
	err = decoder.Decode(&resBody)
	if err != nil {
		r.JSON(rw, http.StatusInternalServerError, map[string]string{
			"status": requests.STATUS_FAILED,
			"error":  err.Error(),
		})
		return
	}

	log.Println(resBody.Application)

	if resBody.Status == requests.STATUS_FAILED {
		r.JSON(rw, http.StatusOK, map[string]string{
			"status": requests.STATUS_FAILED,
			"error":  resBody.Error(),
		})
		return
	}

	// Add application.
	if err = applicationManager.Add(resBody.Application); err != nil {
		r.JSON(rw, http.StatusOK, map[string]string{
			"status": requests.STATUS_FAILED,
			"error":  err.Error(),
		})
		return
	}

	// If the environment for this application is still being provisioned
	// run pings in a goroutine so the application can be updated
	// when it is available.
	app := resBody.Application
	go func() {
		for app.Status != "running" {
			<-time.After(5 * time.Second)
			addr := fmt.Sprintf("%s/applications/%s", config.KenmareAddr, app.ID)
			res, _ := http.Get(addr)
			defer res.Body.Close()

			var resBody applicationRes
			decoder = json.NewDecoder(res.Body)
			decoder.Decode(&resBody)

			log.Println("provisioning status: " + resBody.Application.Status)

			if resBody.Application != nil {
				app = resBody.Application
				applicationManager.UpdateByID(app.ID, app)
			}
		}
	}()

	r.JSON(rw, http.StatusOK, map[string]interface{}{
		"status":      requests.STATUS_SUCCESS,
		"application": app,
	})
}

// getApplicationsHandler gets all applications owned by the developer
// with the provided token.
func getApplicationsHandler(rw http.ResponseWriter, req *http.Request) {
	token := req.FormValue("token")
	apps, err := applicationManager.GetAll(token)
	if err != nil {
		r.JSON(rw, http.StatusOK, map[string]string{
			"status": requests.STATUS_FAILED,
			"error":  err.Error(),
		})
		return
	}

	r.JSON(rw, http.StatusOK, map[string]interface{}{
		"status":       requests.STATUS_FOUND,
		"applications": apps,
	})
}

// updateApplicationHandler updates an application.
func updateApplicationHandler(rw http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	id := vars["id"]
	token := req.FormValue("token")
	if token == "" {
		r.JSON(rw, http.StatusBadRequest, map[string]string{
			"status": requests.STATUS_FAILED,
			"error":  "token required",
		})
	}

	var reqBody schemas.Application
	decoder := json.NewDecoder(req.Body)
	err := decoder.Decode(&reqBody)
	if err != nil {
		r.JSON(rw, http.StatusBadRequest, map[string]string{
			"status": requests.STATUS_FAILED,
			"error":  err.Error(),
		})
		return
	}

	app, err := applicationManager.UpdateByID(id, &reqBody)
	if err != nil {
		r.JSON(rw, http.StatusBadRequest, map[string]string{
			"status": requests.STATUS_FAILED,
			"error":  err.Error(),
		})
		return
	}

	var body bytes.Buffer

	updateBody := applicationReq{
		Name:       app.Name,
		Start:      app.Start,
		Build:      app.Start,
		RemotePath: app.RemotePath,
		LocalPath:  app.LocalPath,
		Token:      token,
	}

	encoder := json.NewEncoder(&body)
	err = encoder.Encode(updateBody)
	if err != nil {
		r.JSON(rw, http.StatusBadRequest, map[string]string{
			"status": requests.STATUS_FAILED,
			"error":  err.Error(),
		})
		return
	}

	addr := fmt.Sprintf("%s/applications/%s", config.KenmareAddr, id)
	request, err := http.NewRequest("PUT", addr, &body)
	if err != nil {
		r.JSON(rw, http.StatusBadRequest, map[string]string{
			"status": requests.STATUS_FAILED,
			"error":  err.Error(),
		})
		return
	}

	request.Header.Set("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(request)
	if err != nil {
		r.JSON(rw, http.StatusBadRequest, map[string]string{
			"status": requests.STATUS_FAILED,
			"error":  err.Error(),
		})
		return
	}

	if res.StatusCode != http.StatusOK {
		r.JSON(rw, http.StatusBadRequest, map[string]string{
			"status": requests.STATUS_FAILED,
			"error":  "failed",
		})
		return
	}

	r.JSON(rw, http.StatusBadRequest, map[string]interface{}{
		"status":      requests.STATUS_SUCCESS,
		"application": app,
	})
}

func getApplicationHandler(rw http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	id := vars["id"]

	app, err := applicationManager.GetByID(id)
	if err != nil {
		r.JSON(rw, http.StatusBadRequest, map[string]string{
			"status": requests.STATUS_FAILED,
			"error":  err.Error(),
		})
		return
	}

	r.JSON(rw, http.StatusOK, map[string]interface{}{
		"status":      requests.STATUS_FOUND,
		"application": app,
	})
}

func removeApplicationHandler(rw http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	id := vars["id"]

	token := req.FormValue("token")
	if token == "" {
		r.JSON(rw, http.StatusBadRequest, map[string]string{
			"status": requests.STATUS_FAILED,
			"error":  "missing fields",
		})
		return
	}

	addr := fmt.Sprintf("%s/applications/%s?token=%s", config.KenmareAddr, id, token)
	req, err := http.NewRequest("DELETE", addr, nil)
	if err != nil {
		r.JSON(rw, http.StatusInternalServerError, map[string]string{
			"status": requests.STATUS_FAILED,
			"error":  err.Error(),
		})
		return
	}

	// Remove the app on kepler.
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		r.JSON(rw, http.StatusInternalServerError, map[string]string{
			"status": requests.STATUS_FAILED,
			"error":  err.Error(),
		})
	}
	defer res.Body.Close()

	removeRes := new(Res)
	decoder := json.NewDecoder(res.Body)
	err = decoder.Decode(removeRes)
	if err != nil {
		r.JSON(rw, http.StatusInternalServerError, map[string]string{
			"status": requests.STATUS_FAILED,
			"error":  err.Error(),
		})
		return
	}

	// If removal failed on kepler respond with it.
	if removeRes.Status == requests.STATUS_FAILED {
		r.JSON(rw, res.StatusCode, map[string]string{
			"status": requests.STATUS_FAILED,
			"error":  removeRes.Error(),
		})
		return
	}

	// Remove locally and stop syncer.
	_, err = applicationManager.RemoveByID(id)
	if err != nil {
		r.JSON(rw, http.StatusBadRequest, map[string]string{
			"status": requests.STATUS_FAILED,
			"error":  err.Error(),
		})
		return
	}

	r.JSON(rw, http.StatusOK, map[string]string{
		"status": requests.STATUS_SUCCESS,
	})
}
