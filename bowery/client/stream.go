// Copyright 2014 Bowery, Inc.
package main

import (
	"encoding/json"
	"io"
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

		data := make([]byte, 128)
		n, err := s.conn.Read(data)
		if err != nil && err != io.EOF {
			s.connect() // Just try to reconnect.
		}
		data = data[:n]

		// No data so just continue.
		if err == io.EOF || len(data) <= 0 {
			continue
		}

		log.Println("data in stream.go#Start", string(data))

		msg := make(map[string]interface{})
		json.Unmarshal(data, &msg)
		msg["appID"] = s.Application.ID

		switch msg["type"] {
		// todo(steve): add plugin errors.
		// case "plugin_error":
		// 	ErrProcessor(s, data)
		case "log":
			LogProcessor(s, data)
		default:
			return
		}

		ssePool.messages <- msg
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
		log.Println("attempting to connect to tcp addr", addr)

		// Ensure previous connection is closed.
		if s.conn != nil {
			s.conn.Close()
		}

		s.conn, err = net.Dial("tcp", addr)
		if err != nil {
			opErr, ok := err.(*net.OpError)
			if ok && (opErr.Op == "read" || opErr.Op == "dial") {
				log.Println("Failed to connect. Retrying...")

				<-time.After(1 * time.Second)
				continue
			}
		}

		log.Println("successfully connected to tcp addr", addr)
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

	log.Println("streamManager#Connect called", app)
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
