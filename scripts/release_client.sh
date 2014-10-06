#!/bin/bash

SOURCE="${BASH_SOURCE[0]}"
while [ -h "$SOURCE" ]; do SOURCE="$(readlink "$SOURCE")"; done
DIR="$( cd -P "$( dirname "$SOURCE" )/.." && pwd )"
CGO_ENABLED=0

# Change into that directory
cd "$DIR/bowery/client"

VERSION=$(cat ../VERSION)
VERSIONDIR="${VERSION}"


# Make sure that if we're killed, we kill all our subprocseses
# trap "kill 0" SIGINT SIGTERM EXIT

# Make sure goxc is installed
go get -u github.com/laher/goxc

# This function builds whatever directory we're in...
echo "--> Cross compling client..."
goxc \
    -tasks-="validate" \
    -d="${DIR}/pkg" \
    -bc="linux windows,386 darwin,amd64" \
    -pv="${VERSION}" \
    go-install \
    xc &> "${DIR}/goxc.log"

cd "$DIR/bowery/client"

echo "--> Downloading shells..."
mkdir -p "${DIR}/pkg" "${DIR}/pkg/darwin_amd64" "${DIR}/pkg/linux_amd64" "${DIR}/pkg/linux_386" "${DIR}/pkg/windows_386" /tmp/atom/

wget -q https://github.com/atom/atom-shell/releases/download/v0.15.4/atom-shell-v0.15.4-darwin-x64.zip -O /tmp/atom/darwin_amd64.zip
unzip -q /tmp/atom/darwin_amd64.zip -d "${DIR}/pkg/darwin_amd64"

wget -q https://github.com/atom/atom-shell/releases/download/v0.15.4/atom-shell-v0.15.4-linux-ia32.zip -O /tmp/atom/linux_386.zip
unzip -q /tmp/atom/linux_386.zip -d "${DIR}/pkg/linux_386"

wget -q https://github.com/atom/atom-shell/releases/download/v0.15.4/atom-shell-v0.15.4-linux-x64.zip -O /tmp/atom/linux_amd64.zip
unzip -q /tmp/atom/linux_amd64.zip -d "${DIR}/pkg/linux_amd64"

wget -q https://github.com/atom/atom-shell/releases/download/v0.15.3/atom-shell-v0.15.3-win32-ia32.zip -O /tmp/atom/windows_386.zip
unzip -q /tmp/atom/windows_386.zip -d "${DIR}/pkg/windows_386"


echo "--> Copying client and app into shells..."
# Move Client For OSX
RESOURCES="${DIR}/pkg/darwin_amd64/Atom.app/Contents/Resources"
mkdir -p "${RESOURCES}/bin"
mkdir -p "${RESOURCES}/app"
cp -rf "${DIR}/pkg/${VERSION}/darwin_amd64/" "${RESOURCES}/bin/"
cp -rf "${DIR}/ui" "${RESOURCES}/"
rm -rf "${RESOURCES}/default_app/"
cp -r "${DIR}/shell/" "${RESOURCES}/app/"
mv "${DIR}/pkg/darwin_amd64/Atom.app" "${DIR}/pkg/darwin_amd64/Bowery.app"
rm "${DIR}/pkg/darwin_amd64/LICENSE"
cat "${DIR}/shell/Info.plist" > "${DIR}/pkg/darwin_amd64/Bowery.app/Contents/Info.plist"
rm -f "${DIR}/pkg/darwin_amd64/Bowery.app/Contents/Resources/atom.icns"
cat "${DIR}/shell/bowery.icns" > "${DIR}/pkg/darwin_amd64/Bowery.app/Contents/Resources/bowery.icns"
codesign -f -vvv -s 'Bowery Software, LLC.' --deep "${DIR}/pkg/darwin_amd64/Bowery.app"


# Move Client for Win32
RESOURCES="${DIR}/pkg/windows_386/resources"
rm -rf "${RESOURCES}/default_app"
mkdir -p "${RESOURCES}/bin"
mkdir -p "${RESOURCES}/app"
cp -rf "${DIR}/pkg/${VERSION}/windows_386/" "${RESOURCES}/bin/"
cp -rf "${DIR}/ui" "${RESOURCES}/"
rm -rf "${RESOURCES}/default_app/"
mkdir -p "${RESOURCES}/app/"
cp -r "${DIR}/shell/" "${RESOURCES}/app/"
rm "${DIR}/pkg/windows_386/LICENSE"

# Move Client for Linux amd64
RESOURCES="${DIR}/pkg/linux_amd64/resources"
rm -rf "${RESOURCES}/default_app"
mkdir -p "${RESOURCES}/bin"
mkdir -p "${RESOURCES}/app"
cp -rf "${DIR}/pkg/${VERSION}/linux_amd64/" "${RESOURCES}/bin/"
cp -rf "${DIR}/ui" "${RESOURCES}/"
rm -rf "${RESOURCES}/default_app/"
mkdir -p "${RESOURCES}/app/"
cp -r "${DIR}/shell/" "${RESOURCES}/app/"
rm "${DIR}/pkg/linux_amd64/LICENSE"

# Move Client for Linux 386
RESOURCES="${DIR}/pkg/linux_386/resources"
rm -rf "${RESOURCES}/default_app"
mkdir -p "${RESOURCES}/bin"
mkdir -p "${RESOURCES}/app"
cp -rf "${DIR}/pkg/${VERSION}/linux_386/" "${RESOURCES}/bin/"
cp -rf "${DIR}/ui" "${RESOURCES}/"
rm -rf "${RESOURCES}/default_app/"
mkdir -p "${RESOURCES}/app/"
cp -r "${DIR}/shell/" "${RESOURCES}/app/"
rm "${DIR}/pkg/linux_386/LICENSE"

# Remove Client Binaries once copy is complete
rm -rf "${DIR}/pkg/${VERSION}"

echo "--> Compressing deliverables..."
mkdir -p "${DIR}/dist/${VERSION}"

cd "${DIR}"
for PLATFORM in $(find "./pkg/" -mindepth 1 -maxdepth 1 -type d); do
  PLATFORM_NAME=$(basename ${PLATFORM})
  ARCHIVE_NAME="${VERSION}_${PLATFORM_NAME}"

  pushd ${PLATFORM}
  tar -czf "${DIR}/dist/${VERSION}/${ARCHIVE_NAME}.tar.gz" ./*
  popd
done

pushd "${DIR}/dist/${VERSION}"
shasum -a256 * > "./${VERSION}_SHA256SUMS"
popd

echo "--> Uploading archives to s3..."
for ARCHIVE in ./dist/${VERSION}/*; do
  ARCHIVE_NAME=$(basename ${ARCHIVE})
  file=$ARCHIVE_NAME
  bucket=desktop.bowery.io
  resource="/${bucket}/${file}"
  contentType="application/octet-stream"
  dateValue=`date -u +"%a, %d %h %Y %T +0000"`
  stringToSign="PUT\n\n${contentType}\n${dateValue}\n${resource}"
  s3Key=${AWS_KEY}
  s3Secret=${AWS_SECRET}
  signature=`echo -en ${stringToSign} | openssl sha1 -hmac ${s3Secret} -binary | base64`
  curl -k\
      -T ${ARCHIVE} \
      -H "Host: ${bucket}.s3.amazonaws.com" \
      -H "Date: ${dateValue}" \
      -H "Content-Type: ${contentType}" \
      -H "Authorization: AWS ${s3Key}:${signature}" \
      https://${bucket}.s3.amazonaws.com/${file}

  echo "* http://desktop.bowery.io/${ARCHIVE_NAME} is available for download."
done
