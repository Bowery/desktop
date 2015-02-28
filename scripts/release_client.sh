#!/usr/bin/env bash
# This script must be run on OS X with xcode, go, brew, and node installed.
STARTTIME=$(date +%s)

# Get the full path to the parent of this script.
source="${BASH_SOURCE[0]}"
while [[ -h "${source}" ]]; do source="$(readlink "${source}")"; done
root="$(cd -P "$(dirname "${source}")/.." && pwd)"
CGO_ENABLED=0


bucket=desktop.bowery.io
s3endpoint="https://${bucket}.s3.amazonaws.com"


# Get the version we're building.
version=$(cat VERSION)
echo "Version: ${version}"

# Setup the directory structure.
rm -rf "${root}/pkg" "${root}/build" "${root}/dist"
mkdir -p /tmp/atom "${root}/pkg" "${root}/build" "${root}/dist"



echo "--> Installing some utilities..."
cd "${root}/scripts"
go build util.go
cd -

if [[ ! -f "$(which 7z)" ]]; then
  brew install p7zip
fi

if [[ ! -f "$(which goxc)" ]]; then
  go get github.com/laher/goxc &> "${root}/debug.log"
fi

echo "--> Downloading shells..."
ver="0.21.2"
url="https://github.com/atom/atom-shell/releases/download/v${ver}/atom-shell-v${ver}"

mkdir -p "${root}/pkg/${version}"

if [[ ! -f "/tmp/atom/${ver}_darwin_amd64.zip" ]]; then
  wget -q "${url}-darwin-x64.zip" -O "/tmp/atom/${ver}_darwin_amd64.zip"
fi
unzip -q "/tmp/atom/${ver}_darwin_amd64.zip" -d "${root}/pkg/${version}/darwin_amd64"

if [[ ! -f "/tmp/atom/${ver}_linux_386.zip" ]]; then
  wget -q "${url}-linux-ia32.zip" -O "/tmp/atom/${ver}_linux_386.zip"
fi
unzip -q "/tmp/atom/${ver}_linux_386.zip" -d "${root}/pkg/${version}/linux_386"

if [[ ! -f "/tmp/atom/${ver}_linux_amd64.zip" ]]; then
  wget -q "${url}-linux-x64.zip" -O "/tmp/atom/${ver}_linux_amd64.zip"
fi
unzip -q "/tmp/atom/${ver}_linux_amd64.zip" -d "${root}/pkg/${version}/linux_amd64"

if [[ ! -f "/tmp/atom/${ver}_windows_386.zip" ]]; then
  wget -q "${url}-win32-ia32.zip" -O "/tmp/atom/${ver}_windows_386.zip"
fi
unzip -q "/tmp/atom/${ver}_windows_386.zip" -d "${root}/pkg/${version}/windows_386"
unzip -q "/tmp/atom/${ver}_windows_386.zip" -d "${root}/pkg/${version}/windows_amd64"

echo "--> Cross compiling client..."
go get ./...
goxc \
-wd="${root}/client" \
-d="${root}/build" \
-bc="linux windows darwin,amd64" \
-pv="${version}" \
xc &> "${root}/debug.log"

echo "--> Cross compiling updater..."
goxc \
-wd="${root}/updater" \
-d="${root}/build" \
-bc="linux windows darwin,amd64" \
-pv="${version}" \
xc &> "${root}/debug.log"

echo "--> Building Atom Shell App"
cd "${root}/shell"
npm install
cd -

echo "--> Moving binaries into bin subdirectory..."
for platform in "${root}/build/${version}/"*; do
  for bin in "${platform}/"*; do
    mkdir -p "${platform}/bin"
    file=$(basename $bin)
    mv "${bin}" "${platform}/bin/${file}"
  done
done

echo "--> Copying shell for each platform..."
for platform in "${root}/build/${version}/"*; do
  cp -rf "${root}/shell" "${platform}"
  mv "${platform}/shell" "${platform}/app"
done


# Darwin specific update stuff.
echo "--> Darwin specific weirdness..."
contents="${root}/build/${version}/darwin_amd64"
mkdir -p "${contents}/Resources"
mv "${contents}/"{bin,app} "${contents}/Resources"
cp -rf "${root}/shell/Info.plist" "${contents}"
cp -rf "${root}/shell/bowery.icns" "${contents}/Resources"

echo "--> Packaging updates as tarballs..."
echo "${version}" > "${root}/dist/VERSION"
for platform in "${root}/build/${version}/"*; do
  platform_name="$(basename "${platform}")"
  archive="${version}_${platform_name}_update.tar.gz"

  pushd "${platform}"
  tar -czf "${root}/dist/${archive}" *
  echo "${s3endpoint}/${archive}" >> "${root}/dist/VERSION"
  popd
done

echo "--> Copying bowery specific code into shells..."
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

  cp -rf "${root}/build/${version}/${platform}/"* "${installDir}"
  rm -rf "${resources}/default_app" "${root}/pkg/${version}/${platform}/"{LICENSE,version}
}

# Move client for Darwin
app="${root}/pkg/${version}/darwin_amd64/Bowery.app"
contents="${app}/Contents"
mv "${root}/pkg/${version}/darwin_amd64/Atom.app" "${app}"
setupAtom "darwin_amd64" "${contents}/Resources" "${contents}"
rm -rf "${contents}/Resources/atom.icns"

# codesign --verbose --deep --force --sign "3rd Party Mac Developer Application: Bowery Software, LLC." "${root}/pkg/${version}/darwin_amd64/Bowery.app"
# codesign --verbose --force --sign "3rd Party Mac Developer Application: Bowery Software, LLC." "${root}/pkg/${version}/darwin_amd64/Bowery.app/Contents/MacOS/Atom"
productbuild --sign "Developer ID Installer: Bowery Software, LLC. (B4B749T38A)" --component "${app}" /Applications "${root}/pkg/${version}/darwin_amd64/bowery.pkg"
spctl -a -vvv --type install "${root}/pkg/${version}/darwin_amd64/bowery.pkg"
rm -rf "${app}"
echo "--> Creating extractors for Linux and Windows..."

# Copies installer scripts and logos in the atom directory.
function setupExtractor {
  platform="${1}"
  installer="${2}"
  icon="${root}/scripts/data/${3}"

  cp -rf "${root}/scripts/data/${installer}" "${icon}" "${root}/pkg/${version}/${platform}"
  cat "${root}/scripts/data/config.txt" | sed -e "s#{{installer}}#${installer}#g" > "${root}/pkg/${version}/${platform}/extractor_config.txt"
}

# Grab the windows sfx for its extractor.
if [[ ! -d /tmp/7zextra ]]; then
  curl -L "http://downloads.sourceforge.net/sevenzip/7z920_extra.7z" > /tmp/7zextra.7z
  7z x -o/tmp/7zextra /tmp/7zextra.7z &> "${root}/debug.log"
fi

# Grab the unix extractor.
if [[ ! -d /tmp/makeself ]]; then
  curl -L "http://megastep.org/makeself/makeself.run" > /tmp/makeself.run
  chmod +x /tmp/makeself.run
  cd /tmp
  ./makeself.run &> "${root}/debug.log"
  cd -
  mv /tmp/makeself-* /tmp/makeself
fi

# Grab the rmmanifest executable for Windows.
if [[ ! -f /tmp/rmmanifest.exe ]]; then
  curl -L "http://desktop.bowery.io.s3.amazonaws.com/rmmanifest.zip" > /tmp/rmmanifest.zip
  cd /tmp
  unzip /tmp/rmmanifest.zip
  cd -
fi

# Setup client for other systems.
setupAtom "linux_386" "${root}/pkg/${version}/linux_386/resources" "{{1}}"
mv "${root}/pkg/${version}/linux_386/atom" "${root}/pkg/${version}/linux_386/bowery"
setupExtractor "linux_386" "install.sh" "logo.png"
/tmp/makeself/makeself.sh "${root}/pkg/${version}/linux_386" "/tmp/${version}_linux_386.run" "Bowery" "./install.sh" &> "${root}/debug.log"
rm -rf "${root}/pkg/${version}/linux_386/"*
cp -f "/tmp/${version}_linux_386.run" "${root}/pkg/${version}/linux_386/bowery.run"
cp -f "${root}/scripts/data/README_linux" "${root}/pkg/${version}/linux_386/README"

setupAtom "linux_amd64" "${root}/pkg/${version}/linux_amd64/resources" "{{1}}"
mv "${root}/pkg/${version}/linux_amd64/atom" "${root}/pkg/${version}/linux_amd64/bowery"
setupExtractor "linux_amd64" "install.sh" "logo.png"
/tmp/makeself/makeself.sh "${root}/pkg/${version}/linux_amd64" "/tmp/${version}_linux_amd64.run" "Bowery" "./install.sh" &> "${root}/debug.log"
rm -rf "${root}/pkg/${version}/linux_amd64/"*
cp -f "/tmp/${version}_linux_amd64.run" "${root}/pkg/${version}/linux_amd64/bowery.run"
cp -f "${root}/scripts/data/README_linux" "${root}/pkg/${version}/linux_amd64/README"

setupAtom "windows_386" "${root}/pkg/${version}/windows_386/resources" "{{1}}"
mv "${root}/pkg/${version}/windows_386/atom.exe" "${root}/pkg/${version}/windows_386/bowery.exe"
setupExtractor "windows_386" "install.bat" "logo.ico"
rm -rf "/tmp/${version}_windows_386.7z"
cp -f "${root}/scripts/data/bowery.exe.manifest" /tmp/rmmanifest.exe "${root}/pkg/${version}/windows_386"
cd "${root}/pkg/${version}/windows_386"
touch bowery.exe.gui {rmmanifest.exe,resources/bin/{client,updater}.exe}.ignore
7z a "/tmp/${version}_windows_386.7z" * &> "${root}/debug.log"
cd -
cat /tmp/7zextra/7zS.sfx "${root}/pkg/${version}/windows_386/extractor_config.txt" "/tmp/${version}_windows_386.7z" > "/tmp/${version}_windows_386.exe"
cp -rf "${root}/pkg/${version}/windows_386" "${root}/pkg/${version}/windows_386_noinstaller"
rm -rf "${root}/pkg/${version}/windows_386/"*
cp -f "/tmp/${version}_windows_386.exe" "${root}/pkg/${version}/windows_386/bowery.exe"
cp -f "${root}/scripts/data/README_windows" "${root}/pkg/${version}/windows_386/README"

setupAtom "windows_amd64" "${root}/pkg/${version}/windows_amd64/resources" "{{1}}"
mv "${root}/pkg/${version}/windows_amd64/atom.exe" "${root}/pkg/${version}/windows_amd64/bowery.exe"
setupExtractor "windows_amd64" "install.bat" "logo.ico"
rm -rf "/tmp/${version}_windows_amd64.7z"
cp -f "${root}/scripts/data/bowery.exe.manifest" /tmp/rmmanifest.exe "${root}/pkg/${version}/windows_amd64"
cd "${root}/pkg/${version}/windows_amd64"
touch bowery.exe.gui {rmmanifest.exe,resources/bin/{client,updater}.exe}.ignore
7z a "/tmp/${version}_windows_amd64.7z" * &> "${root}/debug.log"
cd -
cat /tmp/7zextra/7zS.sfx "${root}/pkg/${version}/windows_amd64/extractor_config.txt" "/tmp/${version}_windows_amd64.7z" > "/tmp/${version}_windows_amd64.exe"
cp -rf "${root}/pkg/${version}/windows_amd64" "${root}/pkg/${version}/windows_amd64_noinstaller"
rm -rf "${root}/pkg/${version}/windows_amd64/"*
cp -f "/tmp/${version}_windows_amd64.exe" "${root}/pkg/${version}/windows_amd64/bowery.exe"
cp -f "${root}/scripts/data/README_windows" "${root}/pkg/${version}/windows_amd64/README"

echo "--> Compressing shells..."
for dir in "${root}/pkg/${version}/"*; do
  platform="$(basename "${dir}")"
  archive="${version}_${platform}.zip"

  cd "${dir}"
  zip -r "${root}/dist/${archive}" * > /dev/null
done

cd "${root}/dist"
shasum -a256 * > "${version}_SHA256SUMS"

echo "--> Uploading archives to s3..."
"${root}/scripts/util" aws "${bucket}" "${root}/dist"
function displaytime {
  local T=$1
  local D=$((T/60/60/24))
  local H=$((T/60/60%24))
  local M=$((T/60%60))
  local S=$((T%60))
  [[ $D > 0 ]] && printf '%d days ' $D
  [[ $H > 0 ]] && printf '%d hours ' $H
  [[ $M > 0 ]] && printf '%d minutes ' $M
  [[ $D > 0 || $H > 0 || $M > 0 ]] && printf 'and '
  printf '%d seconds\n' $S
}

ENDTIME=$(date +%s)
elapsed=$((ENDTIME-STARTTIME))

echo "Done in $(displaytime $elapsed)."
