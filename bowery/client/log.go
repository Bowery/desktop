// Copyright 2013-2014
package main

import (
	"encoding/json"
	"log"
	"net"
	"strings"
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
	l.conn, err = net.Dial("tcp", strings.Split(l.application.RemoteAddr, ":")[0]+":3002")
	if err != nil {
		log.Println(err)
		return
	}

	for {
		// Check if we're done.
		select {
		case <-l.done:
			return
		default:
		}

		data := make([]byte, 512)
		n, _ := l.conn.Read(data)

		if len(string(data[:n])) > 0 {
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
	close(l.done)

	return l.conn.Close()
}
