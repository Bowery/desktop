// Copyright 2013-2014 Bowery, Inc.
package main

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/Bowery/desktop/bowery/agent/plugin"
	"github.com/Bowery/gopackages/sys"
	"github.com/Bowery/gopackages/tar"
)

// 32 MB, same as http.
const httpMaxMem = 32 << 10

var (
	HomeDir    = os.Getenv(sys.HomeVar)
	BoweryDir  = filepath.Join(HomeDir, ".bowery")
	ServiceDir = filepath.Join(BoweryDir, "application")
)

// List of named routes.
var Routes = []*Route{
	&Route{"/", []string{"POST"}, UploadServiceHandler},
	&Route{"/", []string{"PUT"}, UpdateServiceHandler},
	&Route{"/", []string{"GET"}, GetServiceHandler},
	&Route{"/", []string{"DELETE"}, RemoveServiceHandler},
	&Route{"/plugins", []string{"POST"}, UploadPluginHandler},
	&Route{"/plugins", []string{"PUT"}, UpdatePluginHandler},
	&Route{"/plugins", []string{"DELETE"}, RemovePluginHandler},
	&Route{"/healthz", []string{"GET"}, HealthzHandler},
}

// Route is a single named route with a http.HandlerFunc.
type Route struct {
	Path    string
	Methods []string
	Handler http.HandlerFunc
}

// POST /, Upload service code running init steps.
func UploadServiceHandler(rw http.ResponseWriter, req *http.Request) {
	plugin.EmitPluginEvent(plugin.BEFORE_FULL_UPLOAD, "", ServiceDir)
	res := NewResponder(rw, req)
	attach, _, err := req.FormFile("file")
	if err != nil && err != http.ErrMissingFile {
		res.Body["error"] = err.Error()
		res.Send(http.StatusBadRequest)
		return
	}
	init := req.FormValue("init")
	build := req.FormValue("build")
	test := req.FormValue("test")
	start := req.FormValue("start")
	path := req.FormValue("path")
	pathList := strings.Split(path, ":")

	// If target path is specified and path has changed.
	if len(pathList) == 2 && ServiceDir != pathList[1] {
		root := pathList[1]
		if string(root[0]) == "~" {
			root = HomeDir + string(root[1:])
		}
		if string(root[0]) != "/" {
			root = HomeDir + root
		}
		if err := os.Chdir("/"); err != nil {
			res.Body["error"] = err.Error()
			res.Send(http.StatusInternalServerError)
			return
		}
		if err := os.RemoveAll(ServiceDir); err != nil {
			res.Body["error"] = err.Error()
			res.Send(http.StatusInternalServerError)
			return
		}
		err := os.MkdirAll(root, os.ModePerm|os.ModeDir)
		if err == nil {
			err = os.Chdir(root)
		}
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			res.Body["error"] = err.Error()
			res.Send(http.StatusInternalServerError)
			return
		}
		ServiceDir = root
	}

	if attach != nil {
		defer attach.Close()

		err = tar.Untar(attach, ServiceDir)
		if err != nil {
			res.Body["error"] = err.Error()
			res.Send(http.StatusInternalServerError)
			return
		}
	}

	plugin.EmitPluginEvent(plugin.AFTER_FULL_UPLOAD, "", ServiceDir)
	<-Restart(true, true, init, build, test, start)
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
	path := req.FormValue("path")
	typ := req.FormValue("type")
	modeStr := req.FormValue("mode")
	init := req.FormValue("init")
	build := req.FormValue("build")
	test := req.FormValue("test")
	start := req.FormValue("start")

	if path == "" || typ == "" {
		res.Body["error"] = "Missing form fields."
		res.Send(http.StatusBadRequest)
		return
	}
	switch typ {
	case "delete":
		plugin.EmitPluginEvent(plugin.BEFORE_FILE_DELETE, path, ServiceDir)
	case "update":
		plugin.EmitPluginEvent(plugin.BEFORE_FILE_UPDATE, path, ServiceDir)
	case "create":
		plugin.EmitPluginEvent(plugin.BEFORE_FILE_CREATE, path, ServiceDir)
	}
	path = filepath.Join(ServiceDir, filepath.Join(strings.Split(path, "/")...))

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

		dest, err := os.Create(path)
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

		// Set the file permissions if given.
		if modeStr != "" {
			mode, err := strconv.ParseUint(modeStr, 10, 32)
			if err != nil {
				res.Body["error"] = err.Error()
				res.Send(http.StatusBadRequest)
				return
			}

			err = dest.Chmod(os.FileMode(mode))
			if err != nil {
				res.Body["error"] = err.Error()
				res.Send(http.StatusInternalServerError)
				return
			}
		}
	}

	switch typ {
	case "delete":
		plugin.EmitPluginEvent(plugin.AFTER_FILE_DELETE, path, ServiceDir)
	case "update":
		plugin.EmitPluginEvent(plugin.AFTER_FILE_UPDATE, path, ServiceDir)
	case "create":
		plugin.EmitPluginEvent(plugin.AFTER_FILE_CREATE, path, ServiceDir)
	}

	<-Restart(false, true, init, build, test, start)
	res.Body["status"] = "updated"
	res.Send(http.StatusOK)
}

// GET /, Retrieve the service and send it in a gzipped tar.
func GetServiceHandler(rw http.ResponseWriter, req *http.Request) {
	contents, err := tar.Tar(ServiceDir, []string{})
	if err != nil && !os.IsNotExist(err) {
		res := NewResponder(rw, req)
		res.Body["error"] = err.Error()
		res.Send(http.StatusInternalServerError)
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

// DEL /, Remove service files.
func RemoveServiceHandler(rw http.ResponseWriter, req *http.Request) {
	res := NewResponder(rw, req)

	dir, err := os.Open(ServiceDir)
	if err != nil && !os.IsNotExist(err) {
		res.Body["error"] = err.Error()
		res.Send(http.StatusInternalServerError)
		return
	}

	// ServiceDir doesn't exist, nothing to do.
	if err != nil {
		res.Body["status"] = "success"
		res.Send(http.StatusOK)
		return
	}
	defer dir.Close()

	contents, err := dir.Readdir(0)
	if err != nil {
		res.Body["error"] = err.Error()
		res.Send(http.StatusInternalServerError)
		return
	}

	for _, path := range contents {
		err = os.RemoveAll(filepath.Join(ServiceDir, path.Name()))
		if err != nil {
			res.Body["error"] = err.Error()
			res.Send(http.StatusInternalServerError)
			return
		}
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

	name := req.FormValue("name")
	if name == "" {
		res.Body["error"] = "plugin name required"
		res.Send(http.StatusBadRequest)
		return
	}

	hooks := req.FormValue("hooks")
	pluginPath := filepath.Join(plugin.PluginDir, name)
	if attach != nil {
		defer attach.Close()
		if err = tar.Untar(attach, pluginPath); err != nil {
			res.Body["error"] = err.Error()
			res.Send(http.StatusInternalServerError)
			return
		}
	}

	p, err := plugin.NewPlugin(name, hooks)
	if err != nil {
		res.Body["error"] = "unable to create plugin"
		res.Send(http.StatusInternalServerError)
		return
	}

	plugin.AddPlugin(p)

	res.Body["status"] = "success"
	res.Send(http.StatusOK)
}

// PUT /plugins, Updates a plugin
func UpdatePluginHandler(rw http.ResponseWriter, req *http.Request) {
	res := NewResponder(rw, req)

	name := req.FormValue("name")
	isEnabledStr := req.FormValue("isEnabled")
	if name == "" || isEnabledStr == "" {
		res.Body["error"] = "plugin name not provided"
		res.Send(http.StatusBadRequest)
		return
	}

	isEnabled, err := strconv.ParseBool(isEnabledStr)
	if err != nil {
		res.Body["error"] = err.Error()
		res.Send(http.StatusBadRequest)
		return
	}

	if err := plugin.UpdatePlugin(name, isEnabled); err != nil {
		res.Body["error"] = err.Error()
		res.Send(http.StatusBadRequest)
		return
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

// GET /healthz, Return the status of a container
func HealthzHandler(rw http.ResponseWriter, req *http.Request) {
	fmt.Fprintf(rw, "ok")
}
