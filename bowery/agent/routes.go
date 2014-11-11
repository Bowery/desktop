// Copyright 2013-2014 Bowery, Inc.
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/Bowery/desktop/bowery/agent/plugin"
	"github.com/Bowery/gopackages/requests"
	"github.com/Bowery/gopackages/schemas"
	"github.com/Bowery/gopackages/sys"
	"github.com/Bowery/gopackages/tar"
	"github.com/Bowery/gopackages/web"
	"github.com/unrolled/render"
)

// 32 MB, same as http.
const httpMaxMem = 32 << 10

var (
	HomeDir   = os.Getenv(sys.HomeVar)
	BoweryDir = filepath.Join(HomeDir, ".bowery")
)

var r = render.New(render.Options{
	IndentJSON:    true,
	IsDevelopment: true,
})

// List of named routes.
var Routes = []web.Route{
	{"GET", "/", IndexHandler},
	{"POST", "/", UploadServiceHandler},
	{"PUT", "/", UpdateServiceHandler},
	{"DELETE", "/", RemoveServiceHandler},
	{"POST", "/command", RunCommandHandler},
	{"POST", "/commands", RunCommandsHandler},
	{"POST", "/plugins", UploadPluginHandler},
	{"PUT", "/plugins", UpdatePluginHandler},
	{"DELETE", "/plugins", RemovePluginHandler},
	{"GET", "/network", NetworkHandler},
	{"GET", "/healthz", HealthzHandler},
	{"GET", "/_/state/apps", AppStateHandler},
	{"GET", "/_/state/plugins", PluginStateHandler},
}

// runCmdsReq is the request body to execute a command.
type runCmdReq struct {
	AppID string `json:"appID"`
	Cmd   string `json:"cmd"`
}

// runCmdsReq is the request body to execute a number of commands.
type runCmdsReq struct {
	AppID string   `json:"appID"`
	Cmds  []string `json:"cmds"`
}

// GET /, Home page.
func IndexHandler(rw http.ResponseWriter, req *http.Request) {
	id := req.FormValue("id")
	if id == "" {
		fmt.Fprintf(rw, "Bowery Agent v"+VERSION)
		return
	}

	app := Applications[id]
	if app == nil {
		r.JSON(rw, http.StatusBadRequest, map[string]string{
			"status": requests.STATUS_FAILED,
			"error":  "invalid app id",
		})
		return
	}

	contents, err := tar.Tar(app.Path, []string{})
	if err != nil && !os.IsNotExist(err) {
		r.JSON(rw, http.StatusInternalServerError, map[string]string{
			"status": requests.STATUS_FAILED,
			"error":  err.Error(),
		})
		return
	}

	// If the path didn't exist, just provide an empty targz stream.
	if err != nil {
		empty, gzipWriter, tarWriter := tar.NewTarGZ()
		tarWriter.Close()
		gzipWriter.Close()
		contents = empty
	}

	rw.WriteHeader(http.StatusOK)
	io.Copy(rw, contents)
}

// POST /, Upload service code running init steps.
func UploadServiceHandler(rw http.ResponseWriter, req *http.Request) {
	attach, _, err := req.FormFile("file")
	if err != nil && err != http.ErrMissingFile {
		r.JSON(rw, http.StatusBadRequest, map[string]string{
			"status": requests.STATUS_FAILED,
			"error":  err.Error(),
		})
		return
	}
	id := req.FormValue("id")
	init := req.FormValue("init")
	build := req.FormValue("build")
	test := req.FormValue("test")
	start := req.FormValue("start")
	path := req.FormValue("path")

	go logClient.Info("creating application", map[string]interface{}{
		"appID": id,
		"ip":    AgentHost,
	})

	// Create new application.
	app, err := NewApplication(id, init, build, test, start, path)
	if err != nil {
		go logClient.Error(err.Error(), map[string]interface{}{
			"app": app,
			"ip":  AgentHost,
		})
		r.JSON(rw, http.StatusInternalServerError, map[string]string{
			"status": requests.STATUS_FAILED,
			"error":  err.Error(),
		})
		return
	}

	// Set new application, killing any existing cmds created from an app with the same id.
	if oldApp, ok := Applications[id]; ok {
		Kill(oldApp, true)
	}
	Applications[id] = app
	SaveApps()

	plugin.EmitPluginEvent(schemas.BEFORE_FULL_UPLOAD, "", app.Path, app.ID, app.EnabledPlugins)

	if attach != nil {
		defer attach.Close()

		err = tar.Untar(attach, app.Path)
		if err != nil {
			r.JSON(rw, http.StatusInternalServerError, map[string]string{
				"status": requests.STATUS_FAILED,
				"error":  err.Error(),
			})
			return
		}
	}

	plugin.EmitPluginEvent(schemas.AFTER_FULL_UPLOAD, "", app.Path, app.ID, app.EnabledPlugins)
	<-Restart(app, true, true)
	r.JSON(rw, http.StatusOK, map[string]string{
		"status": requests.STATUS_CREATED,
	})
}

// PUT /, Update service.
func UpdateServiceHandler(rw http.ResponseWriter, req *http.Request) {
	err := req.ParseMultipartForm(httpMaxMem)
	if err != nil {
		r.JSON(rw, http.StatusBadRequest, map[string]string{
			"status": requests.STATUS_FAILED,
			"error":  err.Error(),
		})
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

	go logClient.Info("updating application", map[string]interface{}{
		"appID": id,
		"ip":    AgentHost,
	})

	app := Applications[id]
	if app == nil {
		r.JSON(rw, http.StatusBadRequest, map[string]string{
			"status": requests.STATUS_FAILED,
			"error":  "invalid app id",
		})
		return
	}

	// Update application.
	app.Init = init
	app.Build = build
	app.Test = test
	app.Start = start
	SaveApps()

	if path == "" || typ == "" {
		r.JSON(rw, http.StatusBadRequest, map[string]string{
			"status": requests.STATUS_FAILED,
			"error":  "Missing form fields.",
		})
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
			r.JSON(rw, http.StatusInternalServerError, map[string]string{
				"status": requests.STATUS_FAILED,
				"error":  err.Error(),
			})
			return
		}
	} else {
		// Create/Update path in the service.
		var dest *os.File

		if pathType == "dir" {
			err = os.MkdirAll(path, os.ModePerm|os.ModeDir)
			if err != nil {
				r.JSON(rw, http.StatusInternalServerError, map[string]string{
					"status": requests.STATUS_FAILED,
					"error":  err.Error(),
				})
				return
			}
		} else {
			attach, _, err := req.FormFile("file")
			if err != nil {
				if err == http.ErrMissingFile {
					err = errors.New("Missing form fields.")
				}

				r.JSON(rw, http.StatusBadRequest, map[string]string{
					"status": requests.STATUS_FAILED,
					"error":  err.Error(),
				})
				return
			}
			defer attach.Close()

			// Ensure parents exist.
			err = os.MkdirAll(filepath.Dir(path), os.ModePerm|os.ModeDir)
			if err != nil {
				r.JSON(rw, http.StatusInternalServerError, map[string]string{
					"status": requests.STATUS_FAILED,
					"error":  err.Error(),
				})
				return
			}

			dest, err = os.Create(path)
			if err != nil {
				r.JSON(rw, http.StatusInternalServerError, map[string]string{
					"status": requests.STATUS_FAILED,
					"error":  err.Error(),
				})
				return
			}
			defer dest.Close()

			// Copy updated contents to destination.
			_, err = io.Copy(dest, attach)
			if err != nil {
				r.JSON(rw, http.StatusInternalServerError, map[string]string{
					"status": requests.STATUS_FAILED,
					"error":  err.Error(),
				})
				return
			}
		}

		// Set the file permissions if given.
		if modeStr != "" {
			mode, err := strconv.ParseUint(modeStr, 10, 32)
			if err != nil {
				r.JSON(rw, http.StatusBadRequest, map[string]string{
					"status": requests.STATUS_FAILED,
					"error":  err.Error(),
				})
				return
			}

			err = os.Chmod(path, os.FileMode(mode))
			if err != nil {
				r.JSON(rw, http.StatusInternalServerError, map[string]string{
					"status": requests.STATUS_FAILED,
					"error":  err.Error(),
				})
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
	r.JSON(rw, http.StatusOK, map[string]string{
		"status": requests.STATUS_UPDATED,
	})
}

// DELETE /, Remove service.
func RemoveServiceHandler(rw http.ResponseWriter, req *http.Request) {
	id := req.FormValue("id")
	app := Applications[id]
	if app != nil {
		plugin.EmitPluginEvent(schemas.BEFORE_APP_DELETE, "", app.Path, app.ID, app.EnabledPlugins)
		Kill(app, true)
		delete(Applications, id)
		plugin.EmitPluginEvent(schemas.AFTER_APP_DELETE, "", app.Path, app.ID, app.EnabledPlugins)
	}

	go logClient.Info("removing application", map[string]interface{}{
		"appID": id,
		"ip":    AgentHost,
	})

	SaveApps()
	r.JSON(rw, http.StatusOK, map[string]string{
		"status": requests.STATUS_REMOVED,
	})
}

// POST /command, Run a command.
func RunCommandHandler(rw http.ResponseWriter, req *http.Request) {
	body := new(runCmdReq)
	decoder := json.NewDecoder(req.Body)
	err := decoder.Decode(body)
	if err != nil {
		r.JSON(rw, http.StatusBadRequest, map[string]string{
			"status": requests.STATUS_FAILED,
			"error":  err.Error(),
		})
		return
	}

	// Validate body.
	if body.Cmd == "" {
		r.JSON(rw, http.StatusBadRequest, map[string]string{
			"status": requests.STATUS_FAILED,
			"error":  "cmd field is required.",
		})
		return
	}

	go logClient.Info("running command", map[string]interface{}{
		"command": body.Cmd,
		"ip":      AgentHost,
	})

	// Get the data from the optional application.
	path := HomeDir
	var stdout *OutputWriter
	var stderr *OutputWriter
	app := Applications[body.AppID]
	if app != nil {
		path = app.Path
		stdout = app.StdoutWriter
		stderr = app.StderrWriter
	}

	cmd := parseCmd(body.Cmd, path, stdout, stderr)
	go func() {
		err := cmd.Run()
		if err != nil {
			if stderr != nil {
				stderr.Write([]byte(err.Error()))
			}

			log.Println(err)
		}
	}()

	r.JSON(rw, http.StatusOK, map[string]string{
		"status": requests.STATUS_SUCCESS,
	})
}

// POST /commands, Run multiple commands. Do not respond successfully
// until all commands have finished running.
func RunCommandsHandler(rw http.ResponseWriter, req *http.Request) {
	body := new(runCmdsReq)
	decoder := json.NewDecoder(req.Body)
	err := decoder.Decode(body)
	if err != nil {
		r.JSON(rw, http.StatusBadRequest, map[string]string{
			"status": requests.STATUS_FAILED,
			"error":  err.Error(),
		})
		return
	}

	if len(body.Cmds) <= 0 {
		r.JSON(rw, http.StatusBadRequest, map[string]string{
			"status": requests.STATUS_FAILED,
			"error":  "cmds field is required.",
		})
		return
	}

	go logClient.Info("running commands", map[string]interface{}{
		"commands": body.Cmds,
		"ip":       AgentHost,
	})

	// Get the data from the optional application.
	path := HomeDir
	var stdout *OutputWriter
	var stderr *OutputWriter
	app := Applications[body.AppID]
	if app != nil {
		path = app.Path
		stdout = app.StdoutWriter
		stderr = app.StderrWriter
	}

	for _, c := range body.Cmds {
		cmd := parseCmd(c, path, stdout, stderr)
		err := cmd.Run()
		if err != nil {
			if stderr != nil {
				stderr.Write([]byte(err.Error()))
			}

			log.Println(err)
		}
	}

	r.JSON(rw, http.StatusOK, map[string]string{
		"status": requests.STATUS_SUCCESS,
	})
}

// POST /plugins, Upload a plugin
func UploadPluginHandler(rw http.ResponseWriter, req *http.Request) {
	attach, _, err := req.FormFile("file")
	if err != nil && err != http.ErrMissingFile {
		r.JSON(rw, http.StatusBadRequest, map[string]string{
			"status": requests.STATUS_FAILED,
			"error":  err.Error(),
		})
		return
	}

	appID := req.FormValue("appID")
	if appID == "" {
		r.JSON(rw, http.StatusBadRequest, map[string]string{
			"status": requests.STATUS_FAILED,
			"error":  "appID required",
		})
		return
	}

	app := Applications[appID]
	if app == nil {
		r.JSON(rw, http.StatusBadRequest, map[string]string{
			"status": requests.STATUS_FAILED,
			"error":  fmt.Sprintf("no app exists with id %s", appID),
		})
		return
	}

	name := req.FormValue("name")
	if name == "" {
		r.JSON(rw, http.StatusBadRequest, map[string]string{
			"status": requests.STATUS_FAILED,
			"error":  "plugin name required",
		})
		return
	}

	// Create a new plugin.
	hooks := req.FormValue("hooks")
	requirements := req.FormValue("requirements")
	p, err := plugin.NewPlugin(name, hooks, requirements)
	if err != nil {
		r.JSON(rw, http.StatusInternalServerError, map[string]string{
			"status": requests.STATUS_FAILED,
			"error":  err.Error(),
		})
		return
	}

	// Untar the plugin upload.
	pluginPath := filepath.Join(plugin.PluginDir, name)
	if attach != nil {
		defer attach.Close()
		if err = tar.Untar(attach, pluginPath); err != nil {
			r.JSON(rw, http.StatusInternalServerError, map[string]string{
				"status": requests.STATUS_FAILED,
				"error":  err.Error(),
			})
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

	r.JSON(rw, http.StatusOK, map[string]string{
		"status": requests.STATUS_SUCCESS,
	})
}

// PUT /plugins, Updates a plugin
func UpdatePluginHandler(rw http.ResponseWriter, req *http.Request) {
	// TODO (sjkaliski or rm): edit hooks
	appID := req.FormValue("appID")
	name := req.FormValue("name")
	isEnabledStr := req.FormValue("isEnabled")
	if appID == "" || name == "" || isEnabledStr == "" {
		r.JSON(rw, http.StatusBadRequest, map[string]string{
			"status": requests.STATUS_FAILED,
			"error":  "missing fields",
		})
		return
	}

	app := Applications[appID]
	if app == nil {
		r.JSON(rw, http.StatusBadRequest, map[string]string{
			"status": requests.STATUS_FAILED,
			"error":  fmt.Sprintf("no app exists with id %s", appID),
		})
		return
	}

	isEnabled, err := strconv.ParseBool(isEnabledStr)
	if err != nil {
		r.JSON(rw, http.StatusBadRequest, map[string]string{
			"status": requests.STATUS_FAILED,
			"error":  err.Error(),
		})
		return
	}

	// Verify the plugin exists.
	p := plugin.GetPlugin(name)
	if p == nil {
		r.JSON(rw, http.StatusBadRequest, map[string]string{
			"status": requests.STATUS_FAILED,
			"error":  "invalid plugin name",
		})
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

	r.JSON(rw, http.StatusOK, map[string]string{
		"status": requests.STATUS_SUCCESS,
	})
}

// DELETE /plugins?name=PLUGIN_NAME, Removes a plugin
func RemovePluginHandler(rw http.ResponseWriter, req *http.Request) {
	query := req.URL.Query()

	if len(query["name"]) < 1 {
		r.JSON(rw, http.StatusBadRequest, map[string]string{
			"status": requests.STATUS_FAILED,
			"error":  "valid plugin name required",
		})
		return
	}

	pluginName := query["name"][0]

	if err := plugin.RemovePlugin(pluginName); err != nil {
		r.JSON(rw, http.StatusInternalServerError, map[string]string{
			"status": requests.STATUS_FAILED,
			"error":  "unable to remove plugin",
		})
		return
	}

	if err := os.RemoveAll(filepath.Join(plugin.PluginDir, pluginName)); err != nil {
		r.JSON(rw, http.StatusInternalServerError, map[string]string{
			"status": requests.STATUS_FAILED,
			"error":  "unable to remove plugin code",
		})
		return
	}

	r.JSON(rw, http.StatusOK, map[string]string{
		"status": requests.STATUS_SUCCESS,
	})
}

// GET /network, returns network information for an app.
func NetworkHandler(rw http.ResponseWriter, req *http.Request) {
	id := req.FormValue("id")

	app := Applications[id]
	if app == nil {
		r.JSON(rw, http.StatusBadRequest, map[string]string{
			"status": requests.STATUS_FAILED,
			"error":  "invalid app id",
		})
		return
	}

	appNetwork, generic, err := GetNetwork(app)
	if err != nil {
		r.JSON(rw, http.StatusInternalServerError, map[string]string{
			"status": requests.STATUS_FAILED,
			"error":  err.Error(),
		})
		return
	}

	r.JSON(rw, http.StatusOK, map[string]interface{}{
		"status":  requests.STATUS_SUCCESS,
		"app":     appNetwork,
		"generic": generic,
	})
}

// GET /state, Return the current application data.
func AppStateHandler(rw http.ResponseWriter, req *http.Request) {
	data, err := json.Marshal(Applications)
	if err != nil {
		r.JSON(rw, http.StatusInternalServerError, map[string]string{
			"status": requests.STATUS_FAILED,
			"error":  err.Error(),
		})
		return
	}

	rw.Header().Set("Content-Type", "application/json")
	rw.Write(data)
}

func PluginStateHandler(rw http.ResponseWriter, req *http.Request) {
	data, err := json.Marshal(plugin.GetPlugins())
	if err != nil {
		r.JSON(rw, http.StatusInternalServerError, map[string]string{
			"status": requests.STATUS_FAILED,
			"error":  err.Error(),
		})
		return
	}

	rw.Header().Set("Content-Type", "application/json")
	rw.Write(data)
}

// GET /healthz, Return the status of a container
func HealthzHandler(rw http.ResponseWriter, req *http.Request) {
	fmt.Fprintf(rw, "ok")
}
