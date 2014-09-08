// Copyright 2014 Bowery, Inc.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"path/filepath"
	"time"

	"github.com/Bowery/gopackages/config"
	"github.com/Bowery/gopackages/schemas"
	"github.com/Bowery/gopackages/sys"
)

var (
	logDir = filepath.Join(os.Getenv(sys.HomeVar), ".bowery", "logs")
)

type StreamManager struct {
	Streams []*Stream
}

type Stream struct {
	application *schemas.Application
	conn        net.Conn
	done        chan struct{}
}

func NewStreamManager() *StreamManager {
	return &StreamManager{
		Streams: make([]*Stream, 0),
	}
}

func (sm *StreamManager) Connect(app *schemas.Application) {
	s := NewStream(app)
	go s.Start()
	sm.Streams = append(sm.Streams, s)
}

func (sm *StreamManager) Remove(app *schemas.Application) error {
	var (
		i        int
		toDelete bool = false
		s        *Stream
	)

	for i, s = range sm.Streams {
		if s.application.Name == app.Name {
			if err := s.Close(); err != nil {
				return err
			} else {
				toDelete = true
			}
			break
		}
	}

	if toDelete {
		sm.Streams = append(sm.Streams[:i], sm.Streams[i+1:]...)
	}

	return nil
}

func NewStream(app *schemas.Application) *Stream {
	s := new(Stream)
	s.application = app
	s.done = make(chan struct{})
	return s
}

type dataProcessor func(*Stream, []byte) ([]byte, error)

func (s *Stream) Start() {
	var err error
	for {
		log.Println("Attempting to connect.")
		port := config.BoweryAgentProdLogPort
		log.Println(fmt.Sprintf("remote addr: %v, logport: %v", s.application.Location, port))
		s.conn, err = net.Dial("tcp", s.application.Location+":"+port)
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
		case <-s.done:
			return
		default:
		}
		data := make([]byte, 128)
		n, err := s.conn.Read(data)
		if err != nil && err != io.EOF {
			for {
				s.conn.Close()
				s.conn, err = net.Dial("tcp", s.application.Location+":"+config.BoweryAgentProdLogPort)
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
			sMsg := map[string]interface{}{}
			json.Unmarshal(data[:n], &sMsg)
			switch sMsg["type"] {
			// todo(steve): add plugin errors.
			// case "plugin_error":
			// 	ErrProcessor(s, data[:n])
			case "log":
				LogProcessor(s, data[:n])
			default:
				return
			}
			// todo(steve): add web sockets.
			// wsPool.broadcast <- msg
		}
	}
}

func (s *Stream) Close() error {
	var err error

	close(s.done)

	if s.conn != nil {
		err = s.conn.Close()
	}

	return err
}
