#! /usr/bin/env bash
set -e

DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
pushd "${DIR}/build"
  npm install -g grunt-cli
  npm install
  grunt atom-shell-app-builder
popd
