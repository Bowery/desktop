// Copyright 2013-2014 Bowery, Inc.
package main

import (
	"log"
	"net"
)

var (
	// Slice of all connected clients.
	clients []net.Conn
)

// Start a TCP listener on port 3002. Append
// newly connected clients to slice.
func StartTCP() {
	port := "3002"
	if InDevelopment {
		port = "3004"
	}
	listener, err := net.Listen("tcp", ":"+port)
	if err != nil {
		log.Println(err)
		return
	}
	defer listener.Close()

	for {
		conn, _ := listener.Accept()
		clients = append(clients, conn)
	}
}

// TCP.
type TCP struct{}

// NewTCP returns a new TCP.
func NewTCP() *TCP {
	return &TCP{}
}

// Write implements io.Writer writing logs.
func (tcp *TCP) Write(b []byte) (int, error) {
	for _, c := range clients {
		_, err := c.Write(b)
		if err != nil {
			return 0, err
		}
	}
	return len(b), nil
}
