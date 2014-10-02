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
	&Route{"GET", "/logout", logoutHandler},
	&Route{"GET", "/_/sse", sseHandler},
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
		reqBody.AMI = "ami-346ec15c"
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
			"error":  strings.Join(missingFields, ", ") + " is required.",
		})
		return
	}

	localPath := ""
	if reqBody.LocalPath != "" {
		localPath, err = formatLocalDir(reqBody.LocalPath)
		if err != nil {
			r.JSON(rw, http.StatusBadRequest, map[string]string{
				"status": requests.STATUS_FAILED,
				"error":  fmt.Sprintf("%s is not a valid path.", reqBody.LocalPath),
			})
			return
		}
	}
	reqBody.LocalPath = localPath

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

	r.JSON(rw, http.StatusOK, map[string]interface{}{
		"status":      requests.STATUS_SUCCESS,
		"application": resBody.Application,
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

	// Parse the request.
	var reqBody applicationReq
	decoder := json.NewDecoder(req.Body)
	err := decoder.Decode(&reqBody)
	if err != nil {
		log.Println("decoding", err)
		rollbarC.Report(err, map[string]interface{}{
			"id": id,
		})
		r.JSON(rw, http.StatusBadRequest, map[string]string{
			"status": requests.STATUS_FAILED,
			"error":  err.Error(),
		})
		return
	}

	// Ensure that a valid token is provided.
	token := reqBody.Token
	if token == "" {
		log.Println("no token")
		r.JSON(rw, http.StatusBadRequest, map[string]string{
			"status": requests.STATUS_FAILED,
			"error":  "token required",
		})
		return
	}

	// Ensure that a name is provided.
	if reqBody.Name == "" {
		log.Println("no name")
		r.JSON(rw, http.StatusBadRequest, map[string]string{
			"status": requests.STATUS_FAILED,
			"error":  "name required",
		})
		return
	}

	// Parse the local path, allows for ~/ support.
	localPath := ""
	if reqBody.LocalPath != "" {
		localPath, err = formatLocalDir(reqBody.LocalPath)
		if err != nil {
			log.Println("path", err)
			r.JSON(rw, http.StatusBadRequest, map[string]string{
				"status": requests.STATUS_FAILED,
				"error":  fmt.Sprintf("%s is not a valid path.", reqBody.LocalPath),
			})
			return
		}
	}

	// Only the name, start and stop commands, and
	// remote and local paths can be changed.
	changes := &schemas.Application{
		Name:       reqBody.Name,
		Start:      reqBody.Start,
		Build:      reqBody.Build,
		RemotePath: reqBody.RemotePath,
		LocalPath:  localPath,
	}

	// Update the local cache of the application.
	// This resets the watcher and stream for the application.
	app, err := applicationManager.UpdateByID(id, changes)
	if err != nil {
		log.Println("update local", err)
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

	// Send the updates to Kenmare to persist in
	// the database.
	var body bytes.Buffer
	updateBody := applicationReq{
		Name:       app.Name,
		Start:      app.Start,
		Build:      app.Build,
		RemotePath: app.RemotePath,
		LocalPath:  app.LocalPath,
		Token:      reqBody.Token,
	}

	encoder := json.NewEncoder(&body)
	err = encoder.Encode(updateBody)
	if err != nil {
		log.Println("encode", err)
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
		log.Println("update kenmare", err)
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
		log.Println("update kenmare 2", err)
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
		log.Println("parse res", err)
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
		log.Println("failed res", err)
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

	// Respond OK with the application.
	r.JSON(rw, http.StatusOK, map[string]interface{}{
		"status":      requests.STATUS_SUCCESS,
		"application": app,
	})
}

// getApplicationHandler fetches an application.
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

// removeApplicationHandler removes an application.
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

	addr := fmt.Sprintf("%s/applications/%s?%s", config.KenmareAddr, id, req.URL.RawQuery)
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
		"id":     id,
	})
}

// createCommandHandler runs a command on an application agent.
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

	err = DelanceyExec(app, reqBody.Cmd)
	if err != nil {
		r.JSON(rw, http.StatusInternalServerError, map[string]string{
			"status": requests.STATUS_FAILED,
			"error":  err.Error(),
		})
		return
	}

	err = KenmareCreateEvent(app, reqBody.Cmd)
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

// logoutHandler clears the current application state.
func logoutHandler(rw http.ResponseWriter, req *http.Request) {
	applicationManager.Empty()

	r.JSON(rw, http.StatusOK, map[string]string{
		"status": requests.STATUS_SUCCESS,
	})
}

func sseHandler(rw http.ResponseWriter, req *http.Request) {
	f, ok := rw.(http.Flusher)
	if !ok {
		http.Error(rw, "sse not unsupported", http.StatusInternalServerError)
		return
	}

	messageChan := make(chan map[string]interface{})
	ssePool.newClients <- messageChan
	defer func() {
		ssePool.defunctClients <- messageChan
	}()

	rw.Header().Set("Content-Type", "text/event-stream")
	rw.Header().Set("Cache-Control", "no-cache")
	rw.Header().Set("Connection", "keep-alive")

	notify := rw.(http.CloseNotifier).CloseNotify()
	for {
		select {
		case <-notify:
			break
		case msg := <-messageChan:
			log.Println(msg)

			data, err := json.Marshal(msg)
			if err != nil {
				return
			}

			fmt.Fprintf(rw, "data: %v\n\n", string(data))
			f.Flush()
		}
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
