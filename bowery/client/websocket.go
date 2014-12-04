// Copyright 2014 Bowery, Inc.
package main

import (
	"bytes"

	"github.com/gorilla/websocket"
)

// WebSocketIO is an io.ReadWriter that wraps a websocket connection.
type WebSocketIO struct {
	*websocket.Conn
	buf *bytes.Buffer
}

// Connect connects a websocket to do io.
func NewWebSocketIO(conn *websocket.Conn) *WebSocketIO {
	return &WebSocketIO{
		Conn: conn,
		buf:  new(bytes.Buffer),
	}
}

// Write writes to a websocket connection as a binary message.
func (ws *WebSocketIO) Write(b []byte) (int, error) {
	return len(b), ws.WriteMessage(websocket.BinaryMessage, b)
}

// Read reads from buffered websocket connection data.
func (ws *WebSocketIO) Read(b []byte) (int, error) {
	bLen := len(b)

	// Fill the buffer until we have enough data to send back.
	for ws.buf.Len() < bLen {
		_, data, err := ws.ReadMessage()
		if err != nil {
			return 0, err
		}

		_, err = ws.buf.Write(data)
		if err != nil {
			return 0, err
		}
	}

	return ws.buf.Read(b)
}
