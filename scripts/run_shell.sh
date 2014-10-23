#! /usr/bin/env bash
set -e

# Get the full path to the parent of this script.
source="${BASH_SOURCE[0]}"
while [[ -h "${source}" ]]; do source="$(readlink "${source}")"; done
root="$(cd -P "$(dirname "${source}")/.." && pwd)"
build="${root}/build"
mkdir -p "${build}"

cd "${build}"
npm install -g grunt-cli
npm install
grunt download-atom-shell

if [[ ! -d atom-shell/Bowery.app ]]; then
  mv atom-shell/Atom.app atom-shell/Bowery.app
fi
cat "${root}/shell/Info.plist" > atom-shell/Bowery.app/Contents/Info.plist
cp -f "${root}/shell/bowery.icns" atom-shell/Bowery.app/Contents/Resources

./atom-shell/Bowery.app/Contents/MacOS/Atom "${root}/shell"
