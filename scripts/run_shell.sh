#! /usr/bin/env bash
set -e

# Get the full path to the parent of this script.
source="${BASH_SOURCE[0]}"
while [[ -h "${source}" ]]; do source="$(readlink "${source}")"; done
root="$(cd -P "$(dirname "${source}")/.." && pwd)"

cd "${root}/shell"
npm install

if [[ ! -d "${root}/build/Bowery.app" ]]; then
  echo "Downloading Atom Shell..."
  mkdir -p "${root}/build"
  mkdir -p /tmp/shell
  wget -O /tmp/shell/mac.zip https://github.com/atom/atom-shell/releases/download/v0.19.5/atom-shell-v0.19.5-darwin-x64.zip
  unzip -d "${root}/build/" /tmp/shell/mac.zip
  mv "${root}/build/Atom.app" "${root}/build/Bowery.app"
fi

cat "${root}/shell/Info.plist" > "${root}/build/Bowery.app/Contents/Info.plist"
cp -f "${root}/shell/bowery.icns" "${root}/build/Bowery.app/Contents/Resources"
"/${root}/build/Bowery.app/Contents/MacOS/Atom" "${root}/shell"
