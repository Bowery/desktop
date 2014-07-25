// Copyright 2013-2014
package main

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// LogManager manages the tcp connections of a list
// of loggers.
type LogManager struct {
	loggers []*logger
}

// NewLogManager creates a new LogManager made up of
// no loggers.
func NewLogManager() *LogManager {
	return &LogManager{
		loggers: make([]*logger, 0),
	}
}

// Connect creates a new logger for a given application
// and adds it to the LogManager.
func (lm *LogManager) Connect(app *Application) {
	l := NewLogger(app)
	go l.Start()
	lm.loggers = append(lm.loggers, l)
}

// Remove closes the logger for a given app.
func (lm *LogManager) Remove(app *Application) error {
	for idx, logger := range lm.loggers {
		if logger != nil && logger.application.ID == app.ID {
			err := logger.Close()
			if err != nil {
				return err
			}

			lm.loggers[idx] = nil
		}
	}

	return nil
}

// Close closes the loggers connections.
func (lm *LogManager) Close() error {
	for _, logger := range lm.loggers {
		if logger == nil {
			continue
		}

		err := logger.Close()
		if err != nil {
			return err
		}
	}

	return nil
}

// logger manages the tcp connection and data channel
// for an application's logs.
type logger struct {
	application *Application
	conn        net.Conn
	done        chan struct{}
}

// NewLogger creates a new logger for a given
// application.
func NewLogger(app *Application) *logger {
	logger := new(logger)
	logger.application = app
	logger.done = make(chan struct{})
	return logger
}

// Start connects to the application's remote agent's
// tcp listener and broadcasts new data to the wsPool.
func (l *logger) Start() {
	var err error
	for {
		log.Println("Attempting to connect.")
		l.conn, err = net.Dial("tcp", l.application.RemoteAddr+":"+l.application.LogPort)
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

	file, err := os.OpenFile(filepath.Join(logDir, l.application.ID+".log"),
		os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)

	if err != nil {
		log.Println(err)
		return
	}

	output := &syncWriter{File: file}
	write := func(data []byte) error {
		buf := bytes.NewBuffer(data)

		_, err := io.Copy(output, buf)
		if err != nil {
			return err
		}

		return nil
	}

	for {
		// Check if we're done.
		select {
		case <-l.done:
			return
		default:
		}

		data := make([]byte, 512)
		n, err := l.conn.Read(data)
		if err == io.EOF {
			for {
				log.Println("Attempting to connect.")
				l.conn, err = net.Dial("tcp", l.application.RemoteAddr+":"+l.application.LogPort)
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
		}

		if len(string(data[:n])) > 0 {
			write(bytes.Trim(data, "\x00"))
			msg, _ := json.Marshal(map[string]interface{}{
				"appID":   l.application.ID,
				"message": string(data[:n]),
			})
			wsPool.broadcast <- msg
		}
	}
}

// Close closes the loggers connection.
func (l *logger) Close() error {
	var err error
	close(l.done)

	if l.conn != nil {
		err = l.conn.Close()
	}

	return err
}

type syncWriter struct {
	File  *os.File
	mutex sync.Mutex
}

// Write writes the given buffer and syncs to the fs.
func (sw *syncWriter) Write(b []byte) (int, error) {
	sw.mutex.Lock()
	defer sw.mutex.Unlock()

	n, err := sw.File.Write(b)
	if err != nil {
		return n, err
	}

	return n, sw.File.Sync()
}

// Close closes the writer after any writes have completed.
func (sw *syncWriter) Close() error {
	sw.mutex.Lock()
	defer sw.mutex.Unlock()

	return sw.File.Close()
}
