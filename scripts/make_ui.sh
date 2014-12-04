#!/usr/bin/env bash

# Get the full path to the parent of this script.
source="${BASH_SOURCE[0]}"
while [[ -h "${source}" ]]; do source="$(readlink "${source}")"; done
root="$(cd -P "$(dirname "${source}")/.." && pwd)"
cd "${root}"

# Handle installing deps.
if [[ ! -f "$(which myth)" ]]; then
  npm install -g myth
fi

if [[ ! -f "$(which vulcanize)" ]]; then
  npm install -g vulcanize
fi

if [[ ! -f "$(which bower)" ]]; then
  npm install -g bower
fi

if [[ ! -f "$(which grunt)" ]]; then
  npm install -g grunt grunt-cli
fi

myth -v ui/bowery/bowery.css ui/bowery/out.css &> debug.log

cd ui
bower install &> ../debug.log
cd -

if [[ ! -d ui/components/libdot ]]; then
  git clone https://github.com/macton/libdot.git ui/components/libdot
  cd ui/components/libdot
  npm install
  grunt
  mv dist/* .
  cd -
fi

if [[ ! -d ui/components/hterm ]]; then
  git clone https://github.com/macton/hterm.git ui/components/hterm
  cd ui/components/hterm
  mv src/* .
  cd -
fi

mkdir -p bin
vulcanize --verbose --inline ui/bowery/bowery.html -o bin/app.html &> debug.log
vulcanize --verbose --inline ui/bowery/term.html -o bin/term.html &> debug.log
