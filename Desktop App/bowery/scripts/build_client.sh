#!/bin/bash
#
# This script builds the application from source.

# Get the parent directory of where this script is.
set -e

SOURCE="${BASH_SOURCE[0]}"
while [ -h "$SOURCE" ]; do SOURCE="$(readlink "$SOURCE")"; done
DIR="$( cd -P "$( dirname "$SOURCE" )/.." && pwd )"
CGO_ENABLED=0
DIR=$DIR/client

# Change into that directory
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

# Build Client!
echo "--> Building Client..."
cd "${DIR}"
go build \
    -ldflags "${CGO_LDFLAGS} -X main.GitCommit ${GIT_COMMIT}${GIT_DIRTY}" \
    -v \
    -o bin/client
cp bin/client ${GOPATHSINGLE}/bin
DIR=$PWD

# Move it to the XCode proj
while [ ! -e BoweryMenubarApp ]; do cd ..; done
XCODE_DIR=$PWD

BOWERY_DIR=BoweryMenubarApp/Popup/Bowery
mkdir -p $BOWERY_DIR
cp "$DIR/bin/client" $BOWERY_DIR/
cp -r "$DIR/public" $BOWERY_DIR/
cp -r "$DIR/templates" $BOWERY_DIR/
