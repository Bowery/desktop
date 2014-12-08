// Copyright 2014 Bowery, Inc.
// Heavily adapted from https://github.com/kljensen/golang-html5-sse-example.
package main

var ssePool = &pool{
	make(map[chan map[string]interface{}]bool),
	make(chan (chan map[string]interface{})),
	make(chan (chan map[string]interface{})),
	make(chan map[string]interface{}),
}

type pool struct {
	clients        map[chan map[string]interface{}]bool
	newClients     chan chan map[string]interface{}
	defunctClients chan chan map[string]interface{}
	messages       chan map[string]interface{}
}

func (p *pool) run() {
	go func() {
		for {
			select {
			case s := <-p.newClients:
				p.clients[s] = true
			case s := <-p.defunctClients:
				delete(p.clients, s)
			case msg := <-p.messages:
				for s, _ := range p.clients {
					s <- msg
				}
			}
		}
	}()
}
