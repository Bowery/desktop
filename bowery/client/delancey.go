// Copyright 2014 Bowery, Inc.
package main

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
)

// Res is a generic response with status and an error message.
type Res struct {
	Status string `json:"status"`
	Err    string `json:"error"`
}

func (res *Res) Error() string {
	return res.Err
}

// DelanceyUpload sends an upload request including the given file.
func DelanceyUpload(app *Application, file *os.File) error {
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
		err = writer.WriteField("path", app.LocalPath+":"+app.RemotePath)
	}
	if err == nil {
		err = writer.Close()
	}
	if err != nil {
		return err
	}

	res, err := http.Post("http://"+app.RemoteAddr+":"+app.SyncPort, writer.FormDataContentType(), &body)
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
func DelanceyUpdate(app *Application, full, name, status string) error {
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

	req, err := http.NewRequest("PUT", "http://"+app.RemoteAddr+":"+app.SyncPort, &body)
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

// Check checks to see if delancey is running.
func DelanceyCheck(url string) error {
	res, err := http.Get("http://" + url + "/healthz")
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode != 200 {
		return http.ErrNotSupported
	}

	return nil
}
