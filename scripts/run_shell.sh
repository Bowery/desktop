#! /usr/bin/env bash
set -e

# Get the full path to the parent of this script.
source="${BASH_SOURCE[0]}"
while [[ -h "${source}" ]]; do source="$(readlink "${source}")"; done
root="$(cd -P "$(dirname "${source}")/.." && pwd)"
build="${root}/build"

cd "${root}/shell"
npm install

arch="$(uname -p)"
if [[ "${arch}" == "x86_64" ]]; then
  arch="x64"
else
  arch="ia32"
fi

os="linux"
if [[ "${OSTYPE}" == darwin* ]]; then
  os="darwin"
  arch="x64"
fi

if [[ ! -d "${build}/Bowery.app" ]]; then
  echo "Downloading Atom Shell..."
  mkdir -p "${build}"
  mkdir -p /tmp/shell
  wget -O "/tmp/shell/${os}.zip" "https://github.com/atom/atom-shell/releases/download/v0.19.5/atom-shell-v0.19.5-${os}-${arch}.zip"
  if [[ "${os}" == "darwin" ]]; then
    unzip -d "${build}" "/tmp/shell/${os}.zip"
  else
    unzip -d "${build}/Bowery.app" "/tmp/shell/${os}.zip"
  fi
  mv "${build}/Atom.app" "${build}/Bowery.app"
fi

if [[ "${os}" == "darwin" ]]; then
  cat "${root}/shell/Info.plist" > "${build}/Bowery.app/Contents/Info.plist"
  cp -f "${root}/shell/bowery.icns" "${build}/Bowery.app/Contents/Resources"
  "${build}/Bowery.app/Contents/MacOS/Atom" "${root}/shell"
else
  "${build}/Bowery.app/atom" "${root}/shell"
fi
