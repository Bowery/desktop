// Copyright 2013-2014 Bowery, Inc.
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/Bowery/desktop/bowery/agent/plugin"
	"github.com/Bowery/gopackages/schemas"
	"github.com/Bowery/gopackages/sys"
	"github.com/Bowery/gopackages/tar"
)

// 32 MB, same as http.
const httpMaxMem = 32 << 10

var (
	HomeDir   = os.Getenv(sys.HomeVar)
	BoweryDir = filepath.Join(HomeDir, ".bowery")
)

// List of named routes.
var Routes = []*Route{
	&Route{"/", []string{"GET"}, IndexHandler},
	&Route{"/", []string{"POST"}, UploadServiceHandler},
	&Route{"/", []string{"PUT"}, UpdateServiceHandler},
	&Route{"/", []string{"DELETE"}, RemoveServiceHandler},
	&Route{"/command", []string{"POST"}, RunCommandHandler},
	&Route{"/commands", []string{"POST"}, RunCommandsHandler},
	&Route{"/plugins", []string{"POST"}, UploadPluginHandler},
	&Route{"/plugins", []string{"PUT"}, UpdatePluginHandler},
	&Route{"/plugins", []string{"DELETE"}, RemovePluginHandler},
	&Route{"/healthz", []string{"GET"}, HealthzHandler},
	&Route{"/_/state/apps", []string{"GET"}, AppStateHandler},
	&Route{"/_/state/plugins", []string{"GET"}, PluginStateHandler},
}

// Route is a single named route with a http.HandlerFunc.
type Route struct {
	Path    string
	Methods []string
	Handler http.HandlerFunc
}

// GET /, Home page.
func IndexHandler(rw http.ResponseWriter, req *http.Request) {
	fmt.Fprintf(rw, "Bowery Agent v"+VERSION)
}

// POST /, Upload service code running init steps.
func UploadServiceHandler(rw http.ResponseWriter, req *http.Request) {
	res := NewResponder(rw, req)
	attach, _, err := req.FormFile("file")
	if err != nil && err != http.ErrMissingFile {
		res.Body["error"] = err.Error()
		res.Send(http.StatusBadRequest)
		return
	}
	id := req.FormValue("id")
	init := req.FormValue("init")
	build := req.FormValue("build")
	test := req.FormValue("test")
	start := req.FormValue("start")
	path := req.FormValue("path")

	// Create new application.
	app, err := NewApplication(id, init, build, test, start, path)
	if err != nil {
		res.Body["error"] = err.Error()
		res.Send(http.StatusInternalServerError)
		return
	}

	// Set new application, killing any existing cmds created from an app with the same id.
	if oldApp, ok := Applications[id]; ok {
		Kill(oldApp, true)
	}
	Applications[id] = app

	plugin.EmitPluginEvent(schemas.BEFORE_FULL_UPLOAD, "", app.Path, app.ID, app.EnabledPlugins)

	if attach != nil {
		defer attach.Close()

		err = tar.Untar(attach, app.Path)
		if err != nil {
			res.Body["error"] = err.Error()
			res.Send(http.StatusInternalServerError)
			return
		}
	}

	plugin.EmitPluginEvent(schemas.AFTER_FULL_UPLOAD, "", app.Path, app.ID, app.EnabledPlugins)
	<-Restart(app, true, true)
	res.Body["status"] = "created"
	res.Send(http.StatusOK)
}

// PUT /, Update service.
func UpdateServiceHandler(rw http.ResponseWriter, req *http.Request) {
	res := NewResponder(rw, req)
	err := req.ParseMultipartForm(httpMaxMem)
	if err != nil {
		res.Body["error"] = err.Error()
		res.Send(http.StatusBadRequest)
		return
	}
	id := req.FormValue("id")
	pathType := req.FormValue("pathtype")
	path := req.FormValue("path")
	typ := req.FormValue("type")
	modeStr := req.FormValue("mode")
	init := req.FormValue("init")
	build := req.FormValue("build")
	test := req.FormValue("test")
	start := req.FormValue("start")

	app := Applications[id]
	if app == nil {
		res.Body["error"] = "invalid app id"
		res.Send(http.StatusBadRequest)
		return
	}

	// Update application.
	app.Init = init
	app.Build = build
	app.Test = test
	app.Start = start

	if path == "" || typ == "" {
		res.Body["error"] = "Missing form fields."
		res.Send(http.StatusBadRequest)
		return
	}
	switch typ {
	case "delete":
		plugin.EmitPluginEvent(schemas.BEFORE_FILE_DELETE, path, app.Path, app.ID, app.EnabledPlugins)
	case "update":
		plugin.EmitPluginEvent(schemas.BEFORE_FILE_UPDATE, path, app.Path, app.ID, app.EnabledPlugins)
	case "create":
		plugin.EmitPluginEvent(schemas.BEFORE_FILE_CREATE, path, app.Path, app.ID, app.EnabledPlugins)
	}
	path = filepath.Join(app.Path, filepath.Join(strings.Split(path, "/")...))

	if typ == "delete" {
		// Delete path from the service.
		err = os.RemoveAll(path)
		if err != nil {
			res.Body["error"] = err.Error()
			res.Send(http.StatusInternalServerError)
			return
		}
	} else {
		// Create/Update path in the service.
		var dest *os.File

		if pathType == "dir" {
			err = os.MkdirAll(path, os.ModePerm|os.ModeDir)
			if err != nil {
				res.Body["error"] = err.Error()
				res.Send(http.StatusInternalServerError)
				return
			}
		} else {
			attach, _, err := req.FormFile("file")
			if err != nil {
				if err == http.ErrMissingFile {
					err = errors.New("Missing form fields.")
				}

				res.Body["error"] = err.Error()
				res.Send(http.StatusBadRequest)
				return
			}
			defer attach.Close()

			// Ensure parents exist.
			err = os.MkdirAll(filepath.Dir(path), os.ModePerm|os.ModeDir)
			if err != nil {
				res.Body["error"] = err.Error()
				res.Send(http.StatusInternalServerError)
				return
			}

			dest, err = os.Create(path)
			if err != nil {
				res.Body["error"] = err.Error()
				res.Send(http.StatusInternalServerError)
				return
			}
			defer dest.Close()

			// Copy updated contents to destination.
			_, err = io.Copy(dest, attach)
			if err != nil {
				res.Body["error"] = err.Error()
				res.Send(http.StatusInternalServerError)
				return
			}
		}

		// Set the file permissions if given.
		if modeStr != "" {
			mode, err := strconv.ParseUint(modeStr, 10, 32)
			if err != nil {
				res.Body["error"] = err.Error()
				res.Send(http.StatusBadRequest)
				return
			}

			err = os.Chmod(path, os.FileMode(mode))
			if err != nil {
				res.Body["error"] = err.Error()
				res.Send(http.StatusInternalServerError)
				return
			}
		}
	}

	switch typ {
	case "delete":
		plugin.EmitPluginEvent(schemas.AFTER_FILE_DELETE, path, app.Path, app.ID, app.EnabledPlugins)
	case "update":
		plugin.EmitPluginEvent(schemas.AFTER_FILE_UPDATE, path, app.Path, app.ID, app.EnabledPlugins)
	case "create":
		plugin.EmitPluginEvent(schemas.AFTER_FILE_CREATE, path, app.Path, app.ID, app.EnabledPlugins)
	}

	<-Restart(app, false, true)
	res.Body["status"] = "updated"
	res.Send(http.StatusOK)
}

// DELETE /, Remove service.
func RemoveServiceHandler(rw http.ResponseWriter, req *http.Request) {
	res := NewResponder(rw, req)
	id := req.FormValue("id")
	app := Applications[id]
	if app != nil {
		plugin.EmitPluginEvent(schemas.BEFORE_APP_DELETE, "", app.Path, app.ID, app.EnabledPlugins)
		Kill(app, true)
		delete(Applications, id)
		plugin.EmitPluginEvent(schemas.AFTER_APP_DELETE, "", app.Path, app.ID, app.EnabledPlugins)
	}

	res.Body["status"] = "removed"
	res.Send(http.StatusOK)
}

// POST /command, Run a command.
func RunCommandHandler(rw http.ResponseWriter, req *http.Request) {
	res := NewResponder(rw, req)

	body := new(runCmdReq)
	decoder := json.NewDecoder(req.Body)
	err := decoder.Decode(body)
	if err != nil {
		res.Body["error"] = err.Error()
		res.Send(http.StatusBadRequest)
		return
	}

	// Validate body.
	missingFields := make([]string, 0)
	if body.AppID == "" {
		missingFields = append(missingFields, "appID")
	}
	if body.Cmd == "" {
		missingFields = append(missingFields, "cmd")
	}

	if len(missingFields) > 0 {
		res.Body["error"] = "Fields " + strings.Join(missingFields, ", ") + " are required."
		res.Send(http.StatusBadRequest)
		return
	}

	app := Applications[body.AppID]
	if app == nil {
		res.Body["error"] = fmt.Sprintf("no app exists with id %s", body.AppID)
		res.Send(http.StatusBadRequest)
		return
	}

	cmd := parseCmd(body.Cmd, app.Path, app.StdoutWriter, app.StderrWriter)
	go func() {
		if err := cmd.Run(); err != nil {
			app.StderrWriter.Write([]byte(err.Error()))
		}
	}()

	res.Body["status"] = "success"
	res.Send(http.StatusOK)
}

// POST /commands, Run multiple commands. Do not respond successfully
// until all commands have finished running.
func RunCommandsHandler(rw http.ResponseWriter, req *http.Request) {
	res := NewResponder(rw, req)

	body := new(runCmdsReq)
	decoder := json.NewDecoder(req.Body)
	err := decoder.Decode(body)
	if err != nil {
		res.Body["error"] = err.Error()
		res.Send(http.StatusBadRequest)
		return
	}

	for _, c := range body.Cmds {
		cmd := parseCmd(c, sys.HomeVar, nil, nil)
		cmd.Run()
	}

	res.Body["status"] = "success"
	res.Send(http.StatusOK)
}

// POST /plugins, Upload a plugin
func UploadPluginHandler(rw http.ResponseWriter, req *http.Request) {
	res := NewResponder(rw, req)
	attach, _, err := req.FormFile("file")
	if err != nil && err != http.ErrMissingFile {
		res.Body["error"] = err.Error()
		res.Send(http.StatusBadRequest)
		return
	}

	appID := req.FormValue("appID")
	if appID == "" {
		res.Body["error"] = "appID required"
		res.Send(http.StatusBadRequest)
		return
	}

	app := Applications[appID]
	if app == nil {
		res.Body["error"] = fmt.Sprintf("no app exists with id %s", appID)
		res.Send(http.StatusBadRequest)
		return
	}

	name := req.FormValue("name")
	if name == "" {
		res.Body["error"] = "plugin name required"
		res.Send(http.StatusBadRequest)
		return
	}

	// Create a new plugin.
	hooks := req.FormValue("hooks")
	requirements := req.FormValue("requirements")
	p, err := plugin.NewPlugin(name, hooks, requirements)
	if err != nil {
		res.Body["error"] = err.Error()
		res.Send(http.StatusInternalServerError)
		return
	}

	// Untar the plugin upload.
	pluginPath := filepath.Join(plugin.PluginDir, name)
	if attach != nil {
		defer attach.Close()
		if err = tar.Untar(attach, pluginPath); err != nil {
			res.Body["error"] = err.Error()
			res.Send(http.StatusInternalServerError)
			return
		}
	}

	// Add it to the plugin manager.
	if err := plugin.AddPlugin(p); err == nil {
		app.EnabledPlugins = append(app.EnabledPlugins, name)
	}

	// Fire off init and background plugin events.
	go plugin.EmitPluginEvent(schemas.ON_PLUGIN_INIT, "", "", app.ID, app.EnabledPlugins)
	go plugin.EmitPluginEvent(schemas.BACKGROUND, "", "", app.ID, app.EnabledPlugins)

	res.Body["status"] = "success"
	res.Send(http.StatusOK)
}

// PUT /plugins, Updates a plugin
func UpdatePluginHandler(rw http.ResponseWriter, req *http.Request) {
	// TODO (sjkaliski or rm): edit hooks
	res := NewResponder(rw, req)

	appID := req.FormValue("appID")
	name := req.FormValue("name")
	isEnabledStr := req.FormValue("isEnabled")
	if appID == "" || name == "" || isEnabledStr == "" {
		res.Body["error"] = "missing fields"
		res.Send(http.StatusBadRequest)
		return
	}

	app := Applications[appID]
	if app == nil {
		res.Body["error"] = fmt.Sprintf("no app exists with id %s", appID)
		res.Send(http.StatusBadRequest)
		return
	}

	isEnabled, err := strconv.ParseBool(isEnabledStr)
	if err != nil {
		res.Body["error"] = err.Error()
		res.Send(http.StatusBadRequest)
		return
	}

	// Verify the plugin exists.
	p := plugin.GetPlugin(name)
	if p == nil {
		res.Body["error"] = "invalid plugin name"
		res.Send(http.StatusBadRequest)
		return
	}

	// Add/remove from enabled plugins.
	if isEnabled {
		app.EnabledPlugins = append(app.EnabledPlugins, p.Name)

		// Fire off init and background events.
		go plugin.EmitPluginEvent(schemas.ON_PLUGIN_INIT, "", "", app.ID, app.EnabledPlugins)
		go plugin.EmitPluginEvent(schemas.BACKGROUND, "", "", app.ID, app.EnabledPlugins)
	} else {
		for i, ep := range app.EnabledPlugins {
			if ep == p.Name {
				j := i + 1
				app.EnabledPlugins = append(app.EnabledPlugins[:i], app.EnabledPlugins[j:]...)
				break
			}
		}
	}

	res.Body["status"] = "success"
	res.Send(http.StatusOK)
}

// DELETE /plugins?name=PLUGIN_NAME, Removes a plugin
func RemovePluginHandler(rw http.ResponseWriter, req *http.Request) {
	res := NewResponder(rw, req)
	query := req.URL.Query()

	if len(query["name"]) < 1 {
		res.Body["error"] = "valid plugin name required"
		res.Send(http.StatusBadRequest)
		return
	}

	pluginName := query["name"][0]

	if err := plugin.RemovePlugin(pluginName); err != nil {
		res.Body["error"] = "unable to remove plugin"
		res.Send(http.StatusInternalServerError)
		return
	}

	if err := os.RemoveAll(filepath.Join(plugin.PluginDir, pluginName)); err != nil {
		res.Body["error"] = "unable to remove plugin code"
		res.Send(http.StatusInternalServerError)
		return
	}

	res.Body["status"] = "success"
	res.Send(http.StatusOK)
}

// GET /state, Return the current application data.
func AppStateHandler(rw http.ResponseWriter, req *http.Request) {
	data, err := json.Marshal(Applications)
	if err != nil {
		res := NewResponder(rw, req)
		res.Body["error"] = err.Error()
		res.Send(http.StatusInternalServerError)
		return
	}

	rw.Header().Set("Content-Type", "application/json")
	rw.Write(data)
}

func PluginStateHandler(rw http.ResponseWriter, req *http.Request) {
	data, err := json.Marshal(plugin.GetPlugins())
	if err != nil {
		res := NewResponder(rw, req)
		res.Body["error"] = err.Error()
		res.Send(http.StatusInternalServerError)
		return
	}

	rw.Header().Set("Content-Type", "application/json")
	rw.Write(data)
}

// GET /healthz, Return the status of a container
func HealthzHandler(rw http.ResponseWriter, req *http.Request) {
	fmt.Fprintf(rw, "ok")
}
