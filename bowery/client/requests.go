// Copyright 2014 Bowery, Inc.
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Bowery/gopackages/config"
	"github.com/Bowery/gopackages/requests"
	"github.com/Bowery/gopackages/schemas"
	"github.com/Bowery/gopackages/sys"
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
	&Route{"POST", "/commands", createCommandHandler},
	&Route{"GET", "/_/sse/{id}", sseHandler},
}

var r = render.New(render.Options{
	IndentJSON:    true,
	IsDevelopment: true,
})

type commandReq struct {
	AppID string `json:"appID"`
	Cmd   string `json:"cmd"`
	Token string `json:"token"`
}

type applicationReq struct {
	AMI          string `json:"ami"`
	EnvID        string `json:"envID"`
	Token        string `json:"token"`
	Location     string `json:"location"`
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
		rollbarC.Report(err, nil)
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
	missingFields := []string{}
	if reqBody.InstanceType == "" {
		missingFields = append(missingFields, "Instance Type")
	}
	if reqBody.AWSAccessKey == "" {
		missingFields = append(missingFields, "AWS Key")
	}
	if reqBody.AWSSecretKey == "" {
		missingFields = append(missingFields, "AWS Secret")
	}

	if len(missingFields) > 0 {
		r.JSON(rw, http.StatusBadRequest, map[string]string{
			"status": requests.STATUS_FAILED,
			"error":  strings.Join(missingFields, ", ") + " are required.",
		})
		return
	}

	localDir := reqBody.LocalPath
	if localDir != "" {
		reqBody.LocalPath, err = formatLocalDir(localDir)
		if err != nil {
			r.JSON(rw, http.StatusBadRequest, map[string]string{
				"status": requests.STATUS_FAILED,
				"error":  fmt.Sprintf("%s is not a valid path.", localDir),
			})
			return
		}
	}

	// Encode request.
	var data bytes.Buffer
	encoder := json.NewEncoder(&data)
	err = encoder.Encode(reqBody)
	if err != nil {
		rollbarC.Report(err, map[string]interface{}{
			"reqBody": reqBody,
		})
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
		rollbarC.Report(err, map[string]interface{}{
			"reqBody": reqBody,
		})
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
		rollbarC.Report(err, map[string]interface{}{
			"reqBody": reqBody,
		})
		r.JSON(rw, http.StatusInternalServerError, map[string]string{
			"status": requests.STATUS_FAILED,
			"error":  err.Error(),
		})
		return
	}

	if resBody.Status == requests.STATUS_FAILED {
		rollbarC.Report(resBody, map[string]interface{}{
			"reqBody": reqBody,
			"resBody": resBody,
		})
		r.JSON(rw, http.StatusOK, map[string]string{
			"status": requests.STATUS_FAILED,
			"error":  resBody.Error(),
		})
		return
	}

	// Add application.
	if err = applicationManager.Add(resBody.Application); err != nil {
		rollbarC.Report(err, map[string]interface{}{
			"reqBody": reqBody,
			"resBody": resBody,
		})
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
		for app != nil && app.Status != "running" {
			<-time.After(5 * time.Second)
			addr := fmt.Sprintf("%s/applications/%s", config.KenmareAddr, app.ID)
			res, err := http.Get(addr)
			if err != nil {
				continue
			}
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
		rollbarC.Report(err, map[string]interface{}{
			"token": token,
		})
		r.JSON(rw, http.StatusBadRequest, map[string]string{
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

	var reqBody applicationReq
	decoder := json.NewDecoder(req.Body)
	err := decoder.Decode(&reqBody)
	if err != nil {
		rollbarC.Report(err, map[string]interface{}{
			"id": id,
		})
		r.JSON(rw, http.StatusBadRequest, map[string]string{
			"status": requests.STATUS_FAILED,
			"error":  err.Error(),
		})
		return
	}

	token := reqBody.Token
	if token == "" {
		r.JSON(rw, http.StatusBadRequest, map[string]string{
			"status": requests.STATUS_FAILED,
			"error":  "token required",
		})
		return
	}

	changes := &schemas.Application{
		Name:       reqBody.Name,
		Location:   reqBody.Location,
		Start:      reqBody.Start,
		Build:      reqBody.Build,
		RemotePath: reqBody.RemotePath,
		LocalPath:  reqBody.LocalPath,
	}

	app, err := applicationManager.UpdateByID(id, changes)
	if err != nil {
		rollbarC.Report(err, map[string]interface{}{
			"token":   token,
			"id":      id,
			"reqBody": reqBody,
		})
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
		Token:      reqBody.Token,
	}

	encoder := json.NewEncoder(&body)
	err = encoder.Encode(updateBody)
	if err != nil {
		rollbarC.Report(err, map[string]interface{}{
			"token":      token,
			"id":         id,
			"updateBody": updateBody,
		})
		r.JSON(rw, http.StatusBadRequest, map[string]string{
			"status": requests.STATUS_FAILED,
			"error":  err.Error(),
		})
		return
	}

	addr := fmt.Sprintf("%s/applications/%s", config.KenmareAddr, id)
	request, err := http.NewRequest("PUT", addr, &body)
	if err != nil {
		rollbarC.Report(err, map[string]interface{}{
			"token":      token,
			"id":         id,
			"updateBody": updateBody,
		})
		r.JSON(rw, http.StatusBadRequest, map[string]string{
			"status": requests.STATUS_FAILED,
			"error":  err.Error(),
		})
		return
	}

	request.Header.Set("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(request)
	if err != nil {
		rollbarC.Report(err, map[string]interface{}{
			"token":      token,
			"id":         id,
			"updateBody": updateBody,
		})
		r.JSON(rw, http.StatusBadRequest, map[string]string{
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
		rollbarC.Report(err, map[string]interface{}{
			"token":      token,
			"id":         id,
			"updateBody": updateBody,
		})
		r.JSON(rw, http.StatusInternalServerError, map[string]string{
			"status": requests.STATUS_FAILED,
			"error":  err.Error(),
		})
		return
	}

	if resBody.Status == requests.STATUS_FAILED {
		rollbarC.Report(resBody, map[string]interface{}{
			"token":      token,
			"id":         id,
			"updateBody": updateBody,
		})
		r.JSON(rw, http.StatusOK, map[string]string{
			"status": requests.STATUS_FAILED,
			"error":  resBody.Error(),
		})
		return
	}

	r.JSON(rw, http.StatusOK, map[string]interface{}{
		"status":      requests.STATUS_SUCCESS,
		"application": app,
	})
}

func getApplicationHandler(rw http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	id := vars["id"]

	app, err := applicationManager.GetByID(id)
	if err != nil {
		rollbarC.Report(err, map[string]interface{}{
			"id": id,
		})
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

	token := req.URL.Query().Get("token")
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
		rollbarC.Report(err, map[string]interface{}{
			"id":    id,
			"token": token,
		})
		r.JSON(rw, http.StatusInternalServerError, map[string]string{
			"status": requests.STATUS_FAILED,
			"error":  err.Error(),
		})
		return
	}

	// Remove the app on kepler.
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		rollbarC.Report(err, map[string]interface{}{
			"id":    id,
			"token": token,
		})
		r.JSON(rw, http.StatusInternalServerError, map[string]string{
			"status": requests.STATUS_FAILED,
			"error":  err.Error(),
		})
		return
	}
	defer res.Body.Close()

	removeRes := new(Res)
	decoder := json.NewDecoder(res.Body)
	err = decoder.Decode(removeRes)
	if err != nil {
		rollbarC.Report(err, map[string]interface{}{
			"id":    id,
			"token": token,
		})
		r.JSON(rw, http.StatusInternalServerError, map[string]string{
			"status": requests.STATUS_FAILED,
			"error":  err.Error(),
		})
		return
	}

	// If removal failed on kepler respond with it.
	if removeRes.Status == requests.STATUS_FAILED {
		rollbarC.Report(removeRes, map[string]interface{}{
			"id":    id,
			"token": token,
		})
		r.JSON(rw, res.StatusCode, map[string]string{
			"status": requests.STATUS_FAILED,
			"error":  removeRes.Error(),
		})
		return
	}

	// Remove locally and stop syncer.
	_, err = applicationManager.RemoveByID(id)
	if err != nil {
		rollbarC.Report(err, map[string]interface{}{
			"id":    id,
			"token": token,
		})
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

func createCommandHandler(rw http.ResponseWriter, req *http.Request) {
	var reqBody commandReq
	decoder := json.NewDecoder(req.Body)
	err := decoder.Decode(&reqBody)
	if err != nil {
		rollbarC.Report(err, map[string]interface{}{})
		r.JSON(rw, http.StatusInternalServerError, map[string]string{
			"status": requests.STATUS_FAILED,
			"error":  err.Error(),
		})
		return
	}

	token := reqBody.Token
	if token == "" {
		r.JSON(rw, http.StatusBadRequest, map[string]string{
			"status": requests.STATUS_FAILED,
			"error":  "token required",
		})
		return
	}

	if reqBody.Cmd == "" {
		r.JSON(rw, http.StatusBadRequest, map[string]string{
			"status": requests.STATUS_FAILED,
			"error":  "non-empty command required",
		})
		return
	}

	app, err := applicationManager.GetByID(reqBody.AppID)
	if err != nil {
		r.JSON(rw, http.StatusBadRequest, map[string]string{
			"status": requests.STATUS_FAILED,
			"error":  err.Error(),
		})
		return
	}

	// todo(larz): send command to agent, see delancey.go:218.
	err = DelanceyExec(app, reqBody.Cmd)
	if err != nil {
		r.JSON(rw, http.StatusInternalServerError, map[string]string{
			"status": requests.STATUS_FAILED,
			"error":  err.Error(),
		})
		return
	}

	r.JSON(rw, http.StatusOK, map[string]string{
		"status": requests.STATUS_SUCCESS,
	})
}

func sseHandler(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	id := vars["id"]

	f, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "sse not unsupported", http.StatusInternalServerError)
		return
	}

	messageChan := make(chan map[string]interface{})
	ssePool.newClients <- messageChan
	defer func() {
		ssePool.defunctClients <- messageChan
	}()

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	for i := 0; i < 10; i++ {
		msg := <-messageChan
		log.Println(msg)

		if msg["appID"] != id {
			return
		}

		data, err := json.Marshal(msg)
		if err != nil {
			return
		}

		fmt.Fprintf(w, "data: %v\n\n", string(data))
		f.Flush()
	}
}

func formatLocalDir(localDir string) (string, error) {
	if len(localDir) > 0 && localDir[0] == '~' {
		localDir = filepath.Join(os.Getenv(sys.HomeVar), string(localDir[1:]))
	}
	if (len(localDir) > 0 && filepath.Separator == '/' && localDir[0] != '/') ||
		(filepath.Separator != '/' && filepath.VolumeName(localDir) == "") {
		localDir = filepath.Join(os.Getenv(sys.HomeVar), localDir)
	}

	// Validate local path.
	if stat, err := os.Stat(localDir); os.IsNotExist(err) || !stat.IsDir() {
		return "", err
	}

	return localDir, nil
}
