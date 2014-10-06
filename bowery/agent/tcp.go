// Copyright 2013-2014 Bowery, Inc.
package main

import (
	"log"
	"net"

	"github.com/Bowery/gopackages/config"
)

var (
	// Slice of all connected clients.
	clients []net.Conn
)

// Start a TCP listener on port 3002. Append
// newly connected clients to slice.
func StartTCP() {
	port := config.BoweryAgentProdLogPort
	if InDevelopment {
		port = config.BoweryAgentDevLogPort
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
		go logClient.Info("tcp connection accepted", map[string]interface{}{
			"ip": AgentHost,
		})
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
	for i, c := range clients {
		if c == nil {
			continue
		}

		_, err := c.Write(b)
		if err != nil {
			operr, ok := err.(*net.OpError)
			if ok && isDisconnected(operr) {
				clients[i] = nil
				c.Close()
				continue
			}

			return 0, err
		}
	}
	return len(b), nil
}
