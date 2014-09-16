#! /usr/bin/env bash
set -e

DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"

DIR="${DIR}/.."
mkdir -p "${DIR}/build"
pushd "${DIR}/build"
	npm install -g grunt-cli
	npm install
	grunt download-atom-shell
popd

if [ ! -d "${DIR}/build/atom-shell/Bowery.app" ]; then
	mv "${DIR}/build/atom-shell/Atom.app" "${DIR}/build/atom-shell/Bowery.app"
fi
cat "${DIR}/shell/Info.plist" > "${DIR}/build/atom-shell/Bowery.app/Contents/Info.plist"
cp -f "${DIR}/shell/bowery.icns" "${DIR}/build/atom-shell/Bowery.app/Contents/Resources/"

"${DIR}/build/atom-shell/Bowery.app/Contents/MacOS/Atom" "${DIR}/shell"
