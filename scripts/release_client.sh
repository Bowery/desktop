#!/usr/bin/env bash

# Get the parent directory of where this script is.
source="${BASH_SOURCE[0]}"
while [ -h "${source}" ]; do source="$(readlink "${source}")"; done
root="$( cd -P "$( dirname "${source}" )/../" && pwd )"
CGO_ENABLED=0

cd "${root}/scripts"
go build util.go

client="${root}/bowery/client"
echo "Client dir ${client}"
cd "${client}"

# Get the version we're building.
version=$(cat VERSION)
echo "Version: ${version}"

# Setup the directory structure.
pkgdir="${root}/pkg/${version}"
distdir="${root}/dist/${version}"
atom="${pkgdir}/atom"
rm -rf "${pkgdir}"
mkdir -p /tmp/atom "${atom}" "${distdir}"

go get -u github.com/laher/goxc

echo "--> Cross compling client..."
goxc \
  -wd="${client}" \
  -d="${root}/pkg" \
  -bc="linux windows,386 darwin,amd64" \
  -pv="${version}" \
  xc &> "${root}/goxc.log"

echo "--> Downloading shells..."
url="https://github.com/atom/atom-shell/releases/download/v0.17.2/atom-shell-v0.17.2"

wget -q "${url}-darwin-x64.zip" -O /tmp/atom/darwin_amd64.zip
unzip -q /tmp/atom/darwin_amd64.zip -d "${atom}/darwin_amd64"

wget -q "${url}-linux-ia32.zip" -O /tmp/atom/linux_386.zip
unzip -q /tmp/atom/linux_386.zip -d "${atom}/linux_386"

wget -q "${url}-linux-x64.zip" -O /tmp/atom/linux_amd64.zip
unzip -q /tmp/atom/linux_amd64.zip -d "${atom}/linux_amd64"

wget -q "${url}-win32-ia32.zip" -O /tmp/atom/windows_386.zip
unzip -q /tmp/atom/windows_386.zip -d "${atom}/windows_386"

echo "--> Copying client and app into shells..."

# Does the generic setup for an atom platform.
function setupAtom {
  platform="$1"
  resources="$2"

  mkdir -p "${resources}/"{bin,app}
  cp -rf "${pkgdir}/${platform}/"* "${resources}/bin"
  cp -rf "${pkgdir}/${platform}/"* "${resources}/bin"
  cp -rf "${root}/ui" "${resources}"
  cp -rf "${root}/shell/"* "${resources}/app"
  "${root}/scripts/util" json "${resources}/app/package.json" version "${version}"
  rm -rf "${resources}/default_app" "${atom}/${platform}/"{LICENSE,version}
}

# Move client for Darwin
app="${atom}/darwin_amd64/Bowery.app"
resources="${app}/Contents/Resources"
mv "${atom}/darwin_amd64/Atom.app" "${app}"
setupAtom "darwin_amd64" "${resources}"
cp -rf "${root}/shell/Info.plist" "${app}/Contents"
cp -rf "${root}/shell/bowery.icns" "${resources}/bowery.icns"
rm -rf "${resources}/atom.icns"
if [[ "${IDENTITY}" == "" ]]; then
  IDENTITY="Bowery Software, LLC."
fi
codesign -f -vvv -s "${IDENTITY}" --deep "${app}"

# Setup client for other systems.
setupAtom "windows_386" "${atom}/windows_386/resources"
setupAtom "linux_386" "${atom}/linux_386/resources"
setupAtom "linux_amd64" "${atom}/linux_amd64/resources"

echo "--> Compressing shells..."
for dir in $(find "${atom}" -mindepth 1 -maxdepth 1 -type d); do
  platform="$(basename "${dir}")"
  archive="${version}_${platform}.zip"

  cd "${dir}"
  zip -r "${distdir}/${archive}" * > /dev/null
done

cd "${distdir}"
shasum -a256 * > "${version}_SHA256SUMS"

echo "--> Uploading archives to s3..."
for file in "${distdir}/"*; do
  name="$(basename "${file}")"
  bucket=desktop.bowery.io
  resource="/${bucket}/${file}"
  contentType="application/octet-stream"
  dateValue="$(date -u +"%a, %d %h %Y %T +0000")"
  stringToSign="PUT\n\n${contentType}\n${dateValue}\n${resource}"
  s3Key=${AWS_KEY}
  s3Secret=${AWS_SECRET}
  signature=`echo -en ${stringToSign} | openssl sha1 -hmac ${s3Secret} -binary | base64`
  curl -k \
    -T ${file} \
    -H "Host: ${bucket}.s3.amazonaws.com" \
    -H "Date: ${dateValue}" \
    -H "Content-Type: ${contentType}" \
    -H "Authorization: AWS ${s3Key}:${signature}" \
    https://${bucket}.s3.amazonaws.com/${file}

   echo "* http://desktop.bowery.io/${name} is available for download."
done
