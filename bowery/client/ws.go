// Copyright 2013-2014 Bowery, Inc.
// Heavily adapted from Gary Burd's work at
// http://gary.burd.info/go-websocket-chat
package main

import "github.com/gorilla/websocket"

type pool struct {
	connections map[*connection]bool
	broadcast   chan []byte
	register    chan *connection
	unregister  chan *connection
}

var wsPool = pool{
	connections: make(map[*connection]bool),
	broadcast:   make(chan []byte),
	register:    make(chan *connection),
	unregister:  make(chan *connection),
}

func (p *pool) run() {
	for {
		select {
		case conn := <-p.register:
			p.connections[conn] = true
		case conn := <-p.unregister:
			if _, ok := p.connections[conn]; ok {
				delete(p.connections, conn)
				close(conn.send)
			}
		case msg := <-p.broadcast:
			for conn := range p.connections {
				select {
				case conn.send <- msg:
				default:
					delete(p.connections, conn)
					close(conn.send)
				}
			}
		}
	}
}

type connection struct {
	ws   *websocket.Conn
	send chan []byte
}

func (c *connection) reader() {
	for {
		_, msg, err := c.ws.ReadMessage()
		if err != nil {
			break
		}
		wsPool.broadcast <- msg
	}

	c.ws.Close()
}

func (c *connection) writer() {
	for msg := range c.send {
		if err := c.ws.WriteMessage(websocket.TextMessage, msg); err != nil {
			break
		}
	}

	c.ws.Close()
}
