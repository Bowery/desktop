#!/usr/bin/env bash

# Get the full path to the parent of this script.
source="${BASH_SOURCE[0]}"
while [[ -h "${source}" ]]; do source="$(readlink "${source}")"; done
root="$(cd -P "$(dirname "${source}")/.." && pwd)"
CGO_ENABLED=0

client="${root}/bowery/client"
updater="${root}/bowery/updater"
bucket=desktop.bowery.io
s3endpoint="https://${bucket}.s3.amazonaws.com"

npm install &> debug.log

echo "Client dir ${client}"
cd "${client}"

# Get the version we're building.
version=$(cat VERSION)
echo "Version: ${version}"

# Setup the directory structure.
pkgdir="${root}/pkg/${version}"
distdir="${root}/dist/${version}"
atom="${pkgdir}/atom"
updates="${pkgdir}/updates"
rm -rf "${pkgdir}"
mkdir -p /tmp/atom "${atom}" "${updates}" "${distdir}"

echo "--> Cross compiling client..."
goxc \
  -wd="${client}" \
  -d="${root}/pkg" \
  -bc="linux windows,386 darwin,amd64" \
  -pv="${version}" \
  xc &> "${root}/goxc.log"

echo "--> Cross compiling updater..."
goxc \
  -wd="${updater}" \
  -d="${root}/pkg" \
  -bc="linux windows,386 darwin,amd64" \
  -pv="${version}" \
  xc &> "${root}/goxc.log"

echo "--> Downloading shells..."
ver="0.17.2"
url="https://github.com/atom/atom-shell/releases/download/v${ver}/atom-shell-v${ver}"

if [[ ! -f "/tmp/atom/darwin_amd64.zip" ]]; then
  wget -q "${url}-darwin-x64.zip" -O "/tmp/atom/darwin_amd64.zip"
fi
unzip -q "/tmp/atom/darwin_amd64.zip" -d "${atom}/darwin_amd64"

if [[ ! -f "/tmp/atom/linux_386.zip" ]]; then
  wget -q "${url}-linux-ia32.zip" -O /tmp/atom/linux_386.zip
fi
unzip -q "/tmp/atom/linux_386.zip" -d "${atom}/linux_386"

if [[ ! -f "/tmp/atom/linux_amd64.zip" ]]; then
  wget -q "${url}-linux-x64.zip" -O /tmp/atom/linux_amd64.zip
fi
unzip -q "/tmp/atom/linux_amd64.zip" -d "${atom}/linux_amd64"

if [[ ! -f "/tmp/atom/windows_386.zip" ]]; then
  wget -q "${url}-win32-ia32.zip" -O /tmp/atom/windows_386.zip
fi
unzip -q "/tmp/atom/windows_386.zip" -d "${atom}/windows_386"

echo "--> Creating updates for ${version}..."

# Create the update directory with contents for a platform.
function createUpdate {
  platform="${1}"
  dir="${updates}/${platform}"

  mkdir -p "${dir}/"{bin,app}
  cp -rf "${pkgdir}/${platform}/"* "${dir}/bin"
  cp -rf "${root}/ui" "${dir}"
  cp -rf "${root}/shell/"* "${dir}/app"
}

for dir in "${pkgdir}/"*; do
  platform="$(basename "${dir}")"
  if [[ "${platform}" == "atom" ]] || [[ "${platform}" == "updates" ]]; then
    continue
  fi

  createUpdate "${platform}"
done

# Darwin specific update stuff.
contents="${updates}/darwin_amd64"
mkdir -p "${contents}/Resources"
mv "${contents}/"{bin,app,ui} "${contents}/Resources"
cp -rf "${root}/shell/Info.plist" "${contents}"
cp -rf "${root}/shell/bowery.icns" "${contents}/Resources"

# Tar+gzip up the updates and add download urls to the VERSION file.
echo "${version}" > "${distdir}/VERSION"
for platform in "${updates}/"*; do
  platform_name="$(basename "${platform}")"
  archive="${version}_${platform_name}_update.tar.gz"

  pushd "${platform}"
  tar -czf "${distdir}/${archive}" *
  echo "${s3endpoint}/${archive}" >> "${distdir}/VERSION"
  popd
done

echo "--> Copying client and app into shells..."

# Does the generic setup for an atom platform.
function setupAtom {
  platform="${1}"
  resources="${2}"
  installDir="${3}"
  if [[ "${installDir}" == {{*}} ]]; then
    args=("${@}")
    idx="$(echo "${installDir}" | sed -e 's/{{//' -e 's/}}//')"
    installDir="${args[${idx}]}"
  fi

  cp -rf "${updates}/${platform}/"* "${installDir}"
  rm -rf "${resources}/default_app" "${atom}/${platform}/"{LICENSE,version}
}

# Move client for Darwin
app="${atom}/darwin_amd64/Bowery.app"
contents="${app}/Contents"
mv "${atom}/darwin_amd64/Atom.app" "${app}"
setupAtom "darwin_amd64" "${contents}/Resources" "${contents}"
rm -rf "${contents}/Resources/atom.icns"
if [[ "${IDENTITY}" == "" ]]; then
  IDENTITY="Bowery Software, LLC."
fi
productbuild --sign "${IDENTITY}" --component "${app}" /Applications "${atom}/darwin_amd64/bowery.pkg"
rm -rf "${app}"

# Setup client for other systems.
setupAtom "linux_386" "${atom}/linux_386/resources" "{{1}}"
mv "${atom}/linux_386/atom" "${atom}/linux_386/bowery"
setupAtom "linux_amd64" "${atom}/linux_amd64/resources" "{{1}}"
mv "${atom}/linux_amd64/atom" "${atom}/linux_amd64/bowery"
setupAtom "windows_386" "${atom}/windows_386/resources" "{{1}}"
mv "${atom}/windows_386/atom.exe" "${atom}/windows_386/bowery.exe"

echo "--> Compressing shells..."
for dir in "${atom}/"*; do
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
  resource="/${bucket}/${name}"
  contentType="application/octet-stream"
  dateValue="$(date -u +"%a, %d %h %Y %T +0000")"
  stringToSign="PUT\n\n${contentType}\n${dateValue}\n${resource}"
  s3Key=${AWS_KEY}
  s3Secret=${AWS_SECRET}
  signature=`echo -en ${stringToSign} | openssl sha1 -hmac ${s3Secret} -binary | base64`
  curl -k \
   -T ${name} \
   -H "Host: ${bucket}.s3.amazonaws.com" \
   -H "Date: ${dateValue}" \
   -H "Content-Type: ${contentType}" \
   -H "Authorization: AWS ${s3Key}:${signature}" \
   https://${bucket}.s3.amazonaws.com/${name}

  echo "* http://desktop.bowery.io/${name} is available for download."
done
