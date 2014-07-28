#!/bin/bash
echo "Building Bowery Agent!"
if [ -e dev-agent ]; then
  rm dev-agent
fi

GOROOT=/usr/lib/go GOPATH=/root/gopath go get -d
GOROOT=/usr/lib/go GOPATH=/root/gopath go build
