// Copyright 2014 Bowery, Inc.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"time"

	"github.com/Bowery/gopackages/config"
	"github.com/Bowery/gopackages/requests"
	"github.com/Bowery/gopackages/ssh"
	"github.com/Bowery/gopackages/sys"
	"github.com/Bowery/gopackages/update"
	"github.com/Bowery/gopackages/web"
	"github.com/Bowery/kenmare/kenmare"
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"github.com/unrolled/render"
)

var routes = []web.Route{
	{"POST", "/containers", createContainerHandler, false},
	{"DELETE", "/containers/{id}", deleteContainerHandler, false},
	{"GET", "/update/check", checkUpdateHandler, false},
	{"GET", "/update/{version}", doUpdateHandler, false},
	{"GET", "/_/ssh", sshHandler, false},
	{"GET", "/_/sse", sseHandler, false},
}

var renderer = render.New(render.Options{
	IndentJSON:    true,
	IsDevelopment: true,
})

// createContainerHandler requests a container from kenmare.io and initiates the
// sync of the contents of the directory to the container it created.
func createContainerHandler(rw http.ResponseWriter, req *http.Request) {
	var reqBody requests.ContainerReq
	decoder := json.NewDecoder(req.Body)
	err := decoder.Decode(&reqBody)
	if err != nil {
		renderer.JSON(rw, http.StatusBadRequest, map[string]string{
			"status": requests.StatusFailed,
			"error":  err.Error(),
		})
		return
	}

	container, err := kenmare.CreateContainer(reqBody.ImageID)
	if err != nil {
		renderer.JSON(rw, http.StatusInternalServerError, map[string]string{
			"status": requests.StatusFailed,
			"error":  err.Error(),
		})
		return
	}

	containerManager.Add(container)

	renderer.JSON(rw, http.StatusOK, map[string]interface{}{
		"status":    requests.StatusCreated,
		"container": container,
	})
}

// deleteContainerHandler sends a request to kenmare to terminate the container
// and stops local file syncing.
func deleteContainerHandler(rw http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	id := vars["id"]
	skipDelanceyCommit := req.FormValue("skip") != ""

	err := kenmare.DeleteContainer(id, skipDelanceyCommit)
	if err != nil {
		renderer.JSON(rw, http.StatusBadRequest, map[string]string{
			"status": requests.StatusFailed,
			"error":  err.Error(),
		})
		return
	}

	err = containerManager.RemoveByID(id)
	if err != nil {
		renderer.JSON(rw, http.StatusInternalServerError, map[string]string{
			"status": requests.StatusFailed,
			"error":  err.Error(),
		})
		return
	}

	renderer.JSON(rw, http.StatusOK, map[string]string{
		"status": requests.StatusRemoved,
	})
}

func doUpdateHandler(rw http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	ver := vars["version"]
	addr := fmt.Sprintf("%s/%s_%s_%s.zip", config.ClientS3Addr, ver, runtime.GOOS, runtime.GOARCH)
	tmp := filepath.Join(os.TempDir(), "bowery_"+strconv.FormatInt(time.Now().Unix(), 10))

	// This is only needed for darwin.
	if runtime.GOOS != "darwin" {
		renderer.JSON(rw, http.StatusOK, map[string]string{
			"status": requests.StatusUpdated,
		})
		return
	}

	contents, err := update.DownloadVersion(addr)
	if err != nil {
		renderer.JSON(rw, http.StatusInternalServerError, map[string]string{
			"status": requests.StatusFailed,
			"error":  err.Error(),
		})
		return
	}

	for info, body := range contents {
		path := filepath.Join(tmp, info.Name())
		if info.IsDir() {
			continue
		}

		err = os.MkdirAll(filepath.Dir(path), os.ModePerm|os.ModeDir)
		if err != nil {
			renderer.JSON(rw, http.StatusInternalServerError, map[string]string{
				"status": requests.StatusFailed,
				"error":  err.Error(),
			})
			return
		}

		file, err := os.Create(path)
		if err != nil {
			renderer.JSON(rw, http.StatusInternalServerError, map[string]string{
				"status": requests.StatusFailed,
				"error":  err.Error(),
			})
			return
		}
		defer file.Close()

		_, err = io.Copy(file, body)
		if err != nil {
			renderer.JSON(rw, http.StatusInternalServerError, map[string]string{
				"status": requests.StatusFailed,
				"error":  err.Error(),
			})
			return
		}
	}

	go func() {
		cmd := sys.NewCommand("open "+filepath.Join(tmp, "bowery.pkg"), nil)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		err := cmd.Run()
		if err != nil {
			os.Stderr.Write([]byte(err.Error()))
		}
	}()

	renderer.JSON(rw, http.StatusOK, map[string]string{
		"status": requests.StatusUpdated,
	})
}

func checkUpdateHandler(rw http.ResponseWriter, req *http.Request) {
	newVer, _, err := update.GetLatest(config.ClientS3Addr + "/VERSION")
	if err != nil {
		renderer.JSON(rw, http.StatusInternalServerError, map[string]string{
			"status": requests.StatusFailed,
			"error":  err.Error(),
		})
		return
	}

	changed, err := update.OutOfDate(VERSION, newVer)
	if err != nil {
		renderer.JSON(rw, http.StatusInternalServerError, map[string]string{
			"status": requests.StatusFailed,
			"error":  err.Error(),
		})
		return
	}

	body := map[string]string{
		"status": requests.StatusNoUpdate,
	}
	if changed {
		body["status"] = requests.StatusNewUpdate
		body["version"] = newVer
	}

	renderer.JSON(rw, http.StatusOK, body)
}

func sshHandler(rw http.ResponseWriter, req *http.Request) {
	sshClient := &ssh.Client{
		Addr:     net.JoinHostPort(req.FormValue("ip"), config.DelanceySSHPort),
		User:     req.FormValue("user"),
		Password: req.FormValue("password"),
	}
	defer sshClient.Close()

	var rows int
	cols, err := strconv.Atoi(req.FormValue("cols"))
	if err == nil {
		rows, err = strconv.Atoi(req.FormValue("rows"))
	}
	if err != nil {
		rw.WriteHeader(500)
		rw.Write([]byte(err.Error()))
		return
	}
	sshClient.Rows = rows
	sshClient.Cols = cols

	// Setup WebSocket connection.
	upgrader := &websocket.Upgrader{
		CheckOrigin: func(req *http.Request) bool { return true },
	}
	conn, err := upgrader.Upgrade(rw, req, nil)
	if err != nil {
		rw.WriteHeader(500)
		rw.Write([]byte(err.Error()))
		return
	}
	defer conn.Close()

	// Set the stdio wrappers for the connection.
	wsio := NewWebSocketIO(conn)
	sshClient.Stdout = wsio
	sshClient.Stderr = wsio
	sshClient.Stdin = wsio

	err = sshClient.Shell()
	if err != nil && err != websocket.ErrCloseSent {
		conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(1, err.Error()))
	}
}

func sseHandler(rw http.ResponseWriter, req *http.Request) {
	f, ok := rw.(http.Flusher)
	if !ok {
		http.Error(rw, "sse not unsupported", http.StatusInternalServerError)
		return
	}

	messageChan := make(chan map[string]interface{})
	ssePool.NewClients <- messageChan
	defer func() {
		ssePool.DefunctClients <- messageChan
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
