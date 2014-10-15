#!/bin/bash

# Get the full path to the parent of this script.
source="${BASH_SOURCE[0]}"
while [[ -h "${source}" ]]; do source="$(readlink "${source}")"; done
root="$(cd -P "$(dirname "${source}")/.." && pwd)"
cd "${root}/bowery/agent"
mkdir -p "${root}/bin"

echo "--> Installing dependencies..."
go get ./...

echo "--> Building agent..."
go build -o "${root}/bin/agent"
