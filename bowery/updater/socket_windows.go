// Copyright 2013-2014 Bowery, Inc.
package main

import (
	"github.com/natefinch/npipe"
	"net"
	"os"
	"strconv"
)

var socketAddr = `\\.\pipe\bowery-` + strconv.Itoa(os.Getpid()) // WTF is this format lol.

func listenSocket(addr string) (net.Listener, error) {
	return npipe.Listen(addr)
}
