// +build !windows

// Copyright 2013-2014 Bowery, Inc.
package main

import (
	"net"
	"os"
	"path/filepath"
	"strconv"
)

var socketAddr = filepath.Join(os.TempDir(), "bowery-"+strconv.Itoa(os.Getpid())+".sock")

func listenSocket(addr string) (net.Listener, error) {
	return net.Listen("unix", addr)
}
