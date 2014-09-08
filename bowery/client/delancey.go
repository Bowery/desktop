// Copyright 2014 Bowery, Inc.
package main

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/Bowery/gopackages/config"
	"github.com/Bowery/gopackages/schemas"
)

// DelanceyUpload sends an upload request including the given file.
func DelanceyUpload(app *schemas.Application, file *os.File) error {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	// Add file to body.
	part, err := writer.CreateFormFile("file", "upload")
	if err != nil {
		return err
	}
	_, err = io.Copy(part, file)
	if err != nil {
		return err
	}

	// Add ID, commands, and env to body.
	err = writer.WriteField("id", app.ID)
	if err == nil {
		err = writer.WriteField("build", app.Build)
	}
	if err == nil {
		err = writer.WriteField("start", app.Start)
	}
	if err == nil && app.RemotePath != "" {
		// Prepend LocalPath: here so it can recognize the remote path.
		err = writer.WriteField("path", app.LocalPath+"::"+app.RemotePath)
	}
	if err == nil {
		err = writer.Close()
	}
	if err != nil {
		return err
	}

	res, err := http.Post("http://"+net.JoinHostPort(app.Location, config.BoweryAgentProdSyncPort), writer.FormDataContentType(), &body)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	// Decode json response.
	uploadRes := new(Res)
	decoder := json.NewDecoder(res.Body)
	err = decoder.Decode(uploadRes)
	if err != nil {
		return err
	}

	// Created so no error.
	if uploadRes.Status == "created" {
		return nil
	}

	return uploadRes
}

// DelanceyUpdate updates the given name with the status and path.
func DelanceyUpdate(app *schemas.Application, full, name, status string) error {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	err := writer.WriteField("id", app.ID)
	if err == nil {
		err = writer.WriteField("type", status)
	}
	if err == nil {
		err = writer.WriteField("path", path.Join(strings.Split(name, string(filepath.Separator))...))
	}
	if err == nil {
		err = writer.WriteField("build", app.Build)
	}
	if err == nil {
		err = writer.WriteField("start", app.Start)
	}
	if err != nil {
		return err
	}

	// Attach file if update/create.
	if status == "update" || status == "create" {
		file, err := os.Open(full)
		if err != nil {
			return err
		}
		defer file.Close()

		stat, err := file.Stat()
		if err != nil {
			return err
		}

		// Add file mode to write with.
		err = writer.WriteField("mode", strconv.FormatUint(uint64(stat.Mode().Perm()), 10))
		if err != nil {
			return err
		}

		pathType := "file"
		if stat.IsDir() {
			pathType = "dir"
		}
		err = writer.WriteField("pathtype", pathType)
		if err != nil {
			return err
		}

		if pathType == "file" {
			part, err := writer.CreateFormFile("file", "upload")
			if err != nil {
				return err
			}

			_, err = io.Copy(part, file)
			if err != nil {
				return err
			}
		}
	}

	err = writer.Close()
	if err != nil {
		return err
	}

	req, err := http.NewRequest("PUT", "http://"+net.JoinHostPort(app.Location, config.BoweryAgentProdSyncPort), &body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	// Decode json response.
	updateRes := new(Res)
	decoder := json.NewDecoder(res.Body)
	err = decoder.Decode(updateRes)
	if err != nil {
		return err
	}

	// Created so no error.
	if updateRes.Status == "updated" {
		return nil
	}

	return updateRes
}

// DelanceyCheck checks to see if delancey is running.
func DelanceyCheck(url string) error {
	res, err := http.Get("http://" + url + "/healthz")
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return http.ErrNotSupported
	}

	return nil
}

// DelanceyRemove removes an application from it's delancey endpoint.
func DelanceyRemove(app *schemas.Application) error {
	url := net.JoinHostPort(app.Location, config.BoweryAgentProdSyncPort) + "/?id=" + app.ID
	req, err := http.NewRequest("DELETE", "http://"+url, nil)
	if err != nil {
		return err
	}

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	// Decode json response.
	removeRes := new(Res)
	decoder := json.NewDecoder(res.Body)
	err = decoder.Decode(removeRes)
	if err != nil {
		return err
	}

	// Removed so no error.
	if removeRes.Status == "removed" {
		return nil
	}

	return removeRes
}
