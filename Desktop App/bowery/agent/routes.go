// Copyright 2013-2014 Bowery, Inc.
// Contains the routes for satellite.
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

	"github.com/Bowery/gopackages/tar"
)

// 32 MB, same as http.
const httpMaxMem = 32 << 10

// Directory the service lives in.
var HomeDir = "/root/" // default for ubuntu docker container
var ServiceDir = "/application"

// List of named routes.
var Routes = []*Route{
	&Route{"/", []string{"POST"}, UploadServiceHandler},
	&Route{"/", []string{"PUT"}, UpdateServiceHandler},
	&Route{"/", []string{"GET"}, GetServiceHandler},
	&Route{"/", []string{"DELETE"}, RemoveServiceHandler},
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
	env := req.FormValue("env")

	log.Println(path)

	// Parse env data
	envData := map[string]string{}
	if len(env) > 0 {
		if err = json.Unmarshal([]byte(env), &envData); err != nil {
			res.Body["error"] = err.Error()
			res.Send(http.StatusInternalServerError)
			return
		}
	}

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

	<-Restart(true, true, init, build, test, start, envData)
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
	env := req.FormValue("env")

	// Parse env data
	envData := map[string]string{}
	if len(env) > 0 {
		if err = json.Unmarshal([]byte(env), &envData); err != nil {
			res.Body["error"] = err.Error()
			res.Send(http.StatusInternalServerError)
			return
		}
	}

	if path == "" || typ == "" {
		res.Body["error"] = "Missing form fields."
		res.Send(http.StatusBadRequest)
		return
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

	<-Restart(false, true, init, build, test, start, envData)
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

// GET /healthz, Return the status of a container
func HealthzHandler(rw http.ResponseWriter, req *http.Request) {
	fmt.Fprintf(rw, "ok")
}