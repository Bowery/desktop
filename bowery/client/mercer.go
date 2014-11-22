// Copyright 2014 Bowery, Inc.
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net"
	"net/http"

	"github.com/Bowery/gopackages/ignores"
	"github.com/Bowery/gopackages/tar"
)

func init() {
	fmt.Println("loaded mercer.go")
}

type MercerCommands struct {
	Start string `json:"start"`
	Build string `json:"build"`
	Init  string `json:"init"`
}

type mercerRes struct {
	Status   string          `json:"status"`
	Err      string          `json:"error"`
	Commands *MercerCommands `json:"commands"`
}

func MercerUpload(path string) (*MercerCommands, error) {
	fmt.Println("MercerUpload path", path)
	ignoreList, err := ignores.Get(path)
	if err != nil {
		return nil, err
	}

	// Tar the code
	upload, err := tar.Tar(path, ignoreList)
	if err != nil {
		return nil, err
	}

	// Send the Tar to Mercer
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "upload")
	if err != nil {
		return nil, err
	}
	_, err = io.Copy(part, upload)
	if err != nil {
		return nil, err
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}

	fmt.Println("we definitely made the request")
	// TODO (thebyrd) change this to mercer.io instead of localhost once it's working.
	res, err := http.Post("http://"+net.JoinHostPort("localhost", "5000")+"/code", writer.FormDataContentType(), &body)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	// Decode Mercer's json response
	mercerResponse := new(mercerRes)
	decoder := json.NewDecoder(res.Body)
	err = decoder.Decode(mercerResponse)
	if err != nil {
		return nil, err
	}

	if mercerResponse.Status == "success" {
		return mercerResponse.Commands, nil
	} else {
		return nil, errors.New(mercerResponse.Err)
	}
}
