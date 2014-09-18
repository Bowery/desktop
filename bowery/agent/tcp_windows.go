// Copyright 2013-2014 Bowery, Inc.
package main

import (
	"net"
	"syscall"
)

func isDisconnected(operr *net.OpError) bool {
	return operr.Err == syscall.EPIPE || operr.Err == syscall.WSAECONNRESET ||
		operr.Err == syscall.ERROR_BROKEN_PIPE
}
