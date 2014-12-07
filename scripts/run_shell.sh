#! /usr/bin/env bash
set -e

# Get the full path to the parent of this script.
source="${BASH_SOURCE[0]}"
while [[ -h "${source}" ]]; do source="$(readlink "${source}")"; done
root="$(cd -P "$(dirname "${source}")/.." && pwd)"

cat "${root}/shell/Info.plist" > build/Bowery.app/Contents/Info.plist
cp -f "${root}/shell/bowery.icns" build/Bowery.app/Contents/Resources

./build/Bowery.app/Contents/MacOS/Atom "${root}/shell"
