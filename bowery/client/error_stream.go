// Copyright 2014 Bowery, Inc.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"time"
)

// errStream maintains the connection (on a per-app bases) with the agent that then
// streams errors to the client
type errStream struct {
	application *Application
	conn        net.Conn
	done        chan struct{}
}

// appErr is basic error from the agent. Indicates app and the message
type appErr struct {
	AppID   string `json:"appID"`
	Message string `json:"message"`
}

// pluginErr stores information about plugin errors. Including the command that
// triggered them and what plugin was affected
type pluginErr struct {
	appErr
	PluginName string `json:"pluginName"`
	Command    string `json:"command"`
}

type ErrStreamManager struct {
	errStreams []*errStream
}

func NewErrStreamManager() *ErrStreamManager {
	return &ErrStreamManager{
		errStreams: make([]*errStream, 0),
	}
}

func NewErrStream(app *Application) *errStream {
	errStream := new(errStream)
	errStream.application = app
	errStream.done = make(chan struct{})
	return errStream
}

func (es *errStream) Start() {
	var err error
	for {
		log.Println("Attempting to connect.")
		log.Println(fmt.Sprintf("remote addr: %v, logport: %v", es.application.RemoteAddr, es.application.LogPort))
		port := "32058"
		if InDevelopment {
			port = "32057"
		}
		es.conn, err = net.Dial("tcp", es.application.RemoteAddr+":"+port)
		if err != nil {
			if opError, ok := err.(*net.OpError); ok {
				if opError.Op == "read" || opError.Op == "dial" {
					log.Println("Failed to connect. Retrying...")
					<-time.After(1 * time.Second)
					continue
				}
			}
		}
		log.Println("Successfully connected")
		break
	}

	for {
		select {
		case <-es.done:
			return
		default:
		}

		data := make([]byte, 128)
		n, err := es.conn.Read(data)
		if err != nil && err != io.EOF {
			for {
				es.conn.Close()
				es.conn, err = net.Dial("tcp", es.application.RemoteAddr+":"+es.application.LogPort)
				if err != nil {
					if opError, ok := err.(*net.OpError); ok {
						if opError.Op == "read" || opError.Op == "dial" {
							<-time.After(1 * time.Second)
							continue
						}
					}
				}
				break
			}
		}

		if len(string(data[:n])) > 0 {
			perr := &pluginErr{}
			perr.AppID = es.application.ID
			json.Unmarshal(data[:n], perr)
			log.Println(perr)
			msg, _ := json.Marshal(map[string]interface{}{
				"appID":   es.application.ID,
				"message": string(data[:n]),
			})
			wsPool.broadcast <- msg
		}
	}
}

func (esm *ErrStreamManager) Connect(app *Application) {
	a := NewErrStream(app)
	go a.Start()
	esm.errStreams = append(esm.errStreams, a)
}

func (esm *ErrStreamManager) Remove(app *Application) error {
	var (
		i  int
		es *errStream
	)

	for i, es = range esm.errStreams {
		if es.application.Name == app.Name {
			if err := es.Close(); err != nil {
				return err
			}
			break
		}
	}

	esm.errStreams = append(esm.errStreams[:i], esm.errStreams[i+1:]...)
	return nil
}

func (es *errStream) Close() error {
	var err error

	close(es.done)

	if es.conn != nil {
		err = es.conn.Close()
	}

	return err
}
