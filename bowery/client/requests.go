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
<<<<<<< HEAD
=======
	"github.com/Bowery/gopackages/schemas"
>>>>>>> b5ffe84... Add GET /applications/{id} route.
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
	&Route{"GET", "/applications/{id}", getApplicationHandler},
}

var r = render.New(render.Options{
	IndentJSON:    true,
	IsDevelopment: true,
})

type createApplicationReq struct {
	AMI          string `json:"ami"`
	EnvID        string `json:"envID"`
	Token        string `json:"token"`
	InstanceType string `json:"instance_type"`
	AWSAccessKey string `json:"aws_access_key"`
	AWSSecretKey string `json:"aws_secret_key"`
	Ports        string `json:"ports"`
}

// createApplicationHandler creates a new Application which includes:
// persisting it in a db and creating a new env via Kenmare, sending
// application files and commands to the remote agent, and initiating
// file watching.
func createApplicationHandler(rw http.ResponseWriter, req *http.Request) {
	// Parse request.
	var reqBody createApplicationReq
	decoder := json.NewDecoder(req.Body)
	err := decoder.Decode(&reqBody)
	if err != nil {
		r.JSON(rw, http.StatusBadRequest, map[string]string{
			"status": requests.STATUS_FAILED,
			"error":  err.Error(),
		})
		return
	}

	// Validate request.
	if reqBody.AMI == "" || reqBody.InstanceType == "" || reqBody.AWSAccessKey == "" ||
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

	if resBody.Error != "" {
		r.JSON(rw, http.StatusOK, map[string]string{
			"status": requests.STATUS_FAILED,
			"error":  resBody.Error,
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
