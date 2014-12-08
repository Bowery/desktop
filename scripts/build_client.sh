#!/bin/bash

# Get the full path to the parent of this script.
source="${BASH_SOURCE[0]}"
while [[ -h "${source}" ]]; do source="$(readlink "${source}")"; done
root="$(cd -P "$(dirname "${source}")/.." && pwd)"
cd "${root}/client"
mkdir -p "${root}/bin"

echo "--> Installing dependencies..."
go get ./...

# Modify the version so it's greater than the current version.
semver='[^0-9]*\([0-9]*\)[.]\([0-9]*\)[.]\([0-9]*\)\([0-9A-Za-z-]*\)'
version="$(cat VERSION)"
major="$(echo "${version}" | sed -e "s#${semver}#\1.\2.#")"
patch="$(echo "${version}" | sed -e "s#${semver}#\3#")"
((patch++)) # Increment

echo "--> Building client..."
go build -ldflags "-X main.VERSION ${major}${patch}" -o "${root}/bin/client"
