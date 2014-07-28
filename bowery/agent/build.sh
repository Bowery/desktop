#!/bin/bash
echo "Building Bowery Agent!"
rm dev-agent
GOROOT=/usr/lib/go GOPATH=/root/gopath go get -d
GOROOT=/usr/lib/go GOPATH=/root/gopath go build
