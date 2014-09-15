// Copyright 2014 Bowery, Inc.
package main

import (
	"encoding/json"
	"log"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/Bowery/gopackages/config"
	"github.com/Bowery/gopackages/schemas"
	"github.com/Bowery/gopackages/sys"
)

var (
	logDir = filepath.Join(os.Getenv(sys.HomeVar), ".bowery", "logs")
)

// Stream streams logs from a network connection for an application.
type Stream struct {
	Application *schemas.Application
	conn        net.Conn
	decoder     *json.Decoder
	mutex       sync.Mutex
	done        chan struct{}
	isDone      bool
}

// NewStream creates a new stream for an application.
func NewStream(app *schemas.Application) *Stream {
	var mutex sync.Mutex

	return &Stream{
		Application: app,
		mutex:       mutex,
		done:        make(chan struct{}),
	}
}

// Start receives log messages from an application log server and restarts
// if the connection fails.
func (s *Stream) Start() {
	// If previously called Close reset the state.
	s.mutex.Lock()
	if s.isDone {
		s.isDone = false
		s.done = make(chan struct{})
	}
	s.mutex.Unlock()

	s.connect()

	for {
		// Check if we're done.
		select {
		case <-s.done:
			return
		default:
		}

		data := make(map[string]interface{})
		err := s.decoder.Decode(&data)
		if err != nil {
			if isJSONError(err) {
				log.Println("[logs] json error, skipping", err)
			} else {
				log.Println("[logs] network error, reconnecting...")
				s.connect()
			}

			continue
		}
		data["appID"] = s.Application.ID
		log.Println("[logs] data in stream.go#Start", data)

		switch data["type"] {
		// todo(steve): add plugin errors.
		// case "plugin_error":
		// 	ErrProcessor(s, data)
		case "log":
			msg, _ := json.Marshal(data)
			LogProcessor(s, msg)
		default:
			return
		}

		ssePool.messages <- data
	}
}

// Close closes the connections log connection.
func (s *Stream) Close() error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.isDone {
		return nil
	}
	close(s.done)
	s.isDone = true

	conn := s.conn
	s.conn = nil
	if conn != nil {
		return conn.Close()
	}

	return nil
}

// connect will continuously try to connect to the applications log server.
func (s *Stream) connect() {
	var err error

	for {
		addr := net.JoinHostPort(s.Application.Location, config.BoweryAgentProdLogPort)
		log.Println("[logs] attempting to connect to tcp addr", addr)

		// Ensure previous connection is closed.
		if s.conn != nil {
			log.Println("[logs] closing previous connection")
			s.conn.Close()
		}

		s.conn, err = net.Dial("tcp", addr)
		if err != nil {
			opErr, ok := err.(*net.OpError)
			if ok && (opErr.Op == "read" || opErr.Op == "dial") {
				log.Println("[logs] failed to connect. retrying...")

				<-time.After(1 * time.Second)
				continue
			}
		}

		s.decoder = json.NewDecoder(s.conn)
		log.Println("[logs] successfully connected to tcp addr", addr)
		break
	}
}

// StreamManager holds a list of application streams.
type StreamManager struct {
	Streams []*Stream
}

// NewStreamManager creates an empty stream list.
func NewStreamManager() *StreamManager {
	return &StreamManager{
		Streams: make([]*Stream, 0),
	}
}

// Connect starts streaming for a given application.
func (sm *StreamManager) Connect(app *schemas.Application) {
	s := NewStream(app)
	sm.Streams = append(sm.Streams, s)

	log.Println("[logs] streamManager#Connect called", app)
	go s.Start()
}

// Remove stops streaming for a given application.
func (sm *StreamManager) Remove(app *schemas.Application) error {
	for idx, stream := range sm.Streams {
		if stream != nil && stream.Application.ID == app.ID {
			err := stream.Close()
			if err != nil {
				return err
			}

			sm.Streams[idx] = nil
		}
	}

	return nil
}

// Close stops all of the application streams.
func (sm *StreamManager) Close() error {
	for _, stream := range sm.Streams {
		if stream == nil {
			continue
		}

		err := stream.Close()
		if err != nil {
			return err
		}
	}

	return nil
}

// isJSONError detects json unmarshaling errors.
func isJSONError(err error) bool {
	switch err.(type) {
	case *json.InvalidUnmarshalError,
		*json.SyntaxError,
		*json.UnmarshalTypeError,
		*json.UnsupportedValueError:
		return true
	}

	return false
}
