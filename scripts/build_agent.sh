#!/bin/bash
#
# This script builds the agent from source.
#
SOURCE="${BASH_SOURCE[0]}"
while [ -h "$SOURCE" ]; do SOURCE="$(readlink "$SOURCE")"; done
DIR="$( cd -P "$( dirname "$SOURCE" )/.." && pwd )"
CGO_ENABLED=0
ROOT_DIR=$DIR
DIR=$DIR/bowery/agent

cd "$DIR"

# Get the git commit
GIT_COMMIT=$(git rev-parse HEAD)
GIT_DIRTY=$(test -n "`git status --porcelain`" && echo "+CHANGES" || true)

GOPATHSINGLE=${GOPATH%%:*}

# Install dependencies
echo "--> Installing dependencies to speed up builds..."
go get \
  -ldflags "${CGO_LDFLAGS}" \
  ./...

# Build Agent!
echo "--> Building agent..."
cd "${DIR}"
mkdir -p ../../bin

go build \
    -ldflags "${CGO_LDFLAGS} -X main.GitCommit ${GIT_COMMIT}${GIT_DIRTY}" \
    -o ../../bin/agent
