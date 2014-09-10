// Copyright 2013-2014 Bowery, Inc.
package main

import (
	"net"
	"syscall"
)

func isDisconnected(operr *net.OpError) bool {
	return operr.Err == syscall.EPIPE || open.Err == syscall.WSAECONNRESET ||
		open.Err == syscall.ERROR_BROKEN_PIPE
}
