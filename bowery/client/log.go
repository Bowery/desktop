// Copyright 2013-2014
package main

import (
	"encoding/json"
	"log"
	"net"
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
	go l.start()
	lm.loggers = append(lm.loggers, l)
}

// logger manages the tcp connection and data channel
// for an application's logs.
type logger struct {
	application *Application
	conn        net.Conn
	channel     chan []byte
}

// NewLogger creates a new logger for a given
// application.
func NewLogger(app *Application) *logger {
	logger := new(logger)
	logger.application = app
	logger.channel = make(chan []byte)
	return logger
}

// start connects to the application's remote agent's
// tcp listener and broadcasts new data to the wsPool.
func (l *logger) start() {
	var err error
	l.conn, err = net.Dial("tcp", l.application.RemoteAddr+":3002")
	if err != nil {
		log.Println(err)
		return
	}
	defer l.conn.Close()

	go func(ch chan []byte) {
		for {
			data := make([]byte, 512)
			l.conn.Read(data)
			ch <- data
		}
	}(l.channel)

	for {
		select {
		case data := <-l.channel:
			msg, _ := json.Marshal(map[string]interface{}{
				"appID":   l.application.ID,
				"message": string(data),
			})
			wsPool.broadcast <- msg
		}
	}

	return
}
