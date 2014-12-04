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

# Install 7zip for building the Windows extractor.
if [[ ! -f "$(which 7z)" ]]; then
  brew install p7zip
fi

echo "Client dir ${client}"
cd "${client}"

# Get the version we're building.
version=$(cat VERSION)
echo "Version: ${version}"

# Setup the directory structure.
datadir="${root}/scripts/data"
pkgdir="${root}/pkg/${version}"
distdir="${root}/dist/${version}"
atom="${pkgdir}/atom"
updates="${pkgdir}/updates"
rm -rf "${pkgdir}"
mkdir -p /tmp/atom "${atom}" "${updates}" "${distdir}"

cd "${root}/scripts"
go build util.go
cd -

if [[ ! -f "$(which goxc)" ]]; then
  go get github.com/laher/goxc &> "${root}/debug.log"
fi

echo "--> Cross compiling client..."
goxc \
  -wd="${client}" \
  -d="${root}/pkg" \
  -bc="linux windows darwin,amd64" \
  -pv="${version}" \
  xc &> "${root}/debug.log"

echo "--> Cross compiling updater..."
goxc \
  -wd="${updater}" \
  -d="${root}/pkg" \
  -bc="linux windows darwin,amd64" \
  -pv="${version}" \
  xc &> "${root}/debug.log"

echo "--> Downloading shells..."
ver="0.19.1"
url="https://github.com/atom/atom-shell/releases/download/v${ver}/atom-shell-v${ver}"

if [[ ! -f "/tmp/atom/${ver}_darwin_amd64.zip" ]]; then
  wget -q "${url}-darwin-x64.zip" -O "/tmp/atom/${ver}_darwin_amd64.zip"
fi
unzip -q "/tmp/atom/${ver}_darwin_amd64.zip" -d "${atom}/darwin_amd64"

if [[ ! -f "/tmp/atom/${ver}_linux_386.zip" ]]; then
  wget -q "${url}-linux-ia32.zip" -O "/tmp/atom/${ver}_linux_386.zip"
fi
unzip -q "/tmp/atom/${ver}_linux_386.zip" -d "${atom}/linux_386"

if [[ ! -f "/tmp/atom/${ver}_linux_amd64.zip" ]]; then
  wget -q "${url}-linux-x64.zip" -O "/tmp/atom/${ver}_linux_amd64.zip"
fi
unzip -q "/tmp/atom/${ver}_linux_amd64.zip" -d "${atom}/linux_amd64"

if [[ ! -f "/tmp/atom/${ver}_windows_386.zip" ]]; then
  wget -q "${url}-win32-ia32.zip" -O "/tmp/atom/${ver}_windows_386.zip"
fi
unzip -q "/tmp/atom/${ver}_windows_386.zip" -d "${atom}/windows_386"
unzip -q "/tmp/atom/${ver}_windows_386.zip" -d "${atom}/windows_amd64"

echo "--> Creating updates for ${version}..."

# Create the update directory with contents for a platform.
function createUpdate {
  platform="${1}"
  dir="${updates}/${platform}"

  cp -f "${root}/bin/app.html" "${pkgdir}/${platform}"
  cp -f "${root}/bin/logs.html" "${pkgdir}/${platform}"
  mkdir -p "${dir}/"{bin,app}
  cp -rf "${pkgdir}/${platform}/"* "${dir}/bin"
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
mv "${contents}/"{bin,app} "${contents}/Resources"
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

echo "--> Creating extractors for Linux and Windows..."

# Copies installer scripts and logos in the atom directory.
function setupExtractor {
  platform="${1}"
  installer="${2}"
  icon="${datadir}/${3}"

  cp -rf "${datadir}/${installer}" "${icon}" "${atom}/${platform}"
  cat "${datadir}/config.txt" | sed -e "s#{{installer}}#${installer}#g" > "${atom}/${platform}/extractor_config.txt"
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
setupAtom "linux_386" "${atom}/linux_386/resources" "{{1}}"
mv "${atom}/linux_386/atom" "${atom}/linux_386/bowery"
setupExtractor "linux_386" "install.sh" "logo.png"
/tmp/makeself/makeself.sh "${atom}/linux_386" "/tmp/${version}_linux_386.run" "Bowery" "./install.sh" &> "${root}/debug.log"
rm -rf "${atom}/linux_386/"*
cp -f "/tmp/${version}_linux_386.run" "${atom}/linux_386/bowery.run"
cp -f "${datadir}/README_linux" "${atom}/linux_386/README"

setupAtom "linux_amd64" "${atom}/linux_amd64/resources" "{{1}}"
mv "${atom}/linux_amd64/atom" "${atom}/linux_amd64/bowery"
setupExtractor "linux_amd64" "install.sh" "logo.png"
/tmp/makeself/makeself.sh "${atom}/linux_amd64" "/tmp/${version}_linux_amd64.run" "Bowery" "./install.sh" &> "${root}/debug.log"
rm -rf "${atom}/linux_amd64/"*
cp -f "/tmp/${version}_linux_amd64.run" "${atom}/linux_amd64/bowery.run"
cp -f "${datadir}/README_linux" "${atom}/linux_amd64/README"

setupAtom "windows_386" "${atom}/windows_386/resources" "{{1}}"
mv "${atom}/windows_386/atom.exe" "${atom}/windows_386/bowery.exe"
setupExtractor "windows_386" "install.bat" "logo.ico"
rm -rf "/tmp/${version}_windows_386.7z"
cp -f "${datadir}/bowery.exe.manifest" /tmp/rmmanifest.exe "${atom}/windows_386"
cd "${atom}/windows_386"
7z a "/tmp/${version}_windows_386.7z" * &> "${root}/debug.log"
cd -
cat /tmp/7zextra/7zS.sfx "${atom}/windows_386/extractor_config.txt" "/tmp/${version}_windows_386.7z" > "/tmp/${version}_windows_386.exe"
rm -rf "${atom}/windows_386/"*
cp -f "/tmp/${version}_windows_386.exe" "${atom}/windows_386/bowery.exe"
cp -f "${datadir}/README_windows" "${atom}/windows_386/README"

setupAtom "windows_amd64" "${atom}/windows_amd64/resources" "{{1}}"
mv "${atom}/windows_amd64/atom.exe" "${atom}/windows_amd64/bowery.exe"
setupExtractor "windows_amd64" "install.bat" "logo.ico"
rm -rf "/tmp/${version}_windows_amd64.7z"
cp -f "${datadir}/bowery.exe.manifest" /tmp/rmmanifest.exe "${atom}/windows_amd64"
cd "${atom}/windows_amd64"
7z a "/tmp/${version}_windows_amd64.7z" * &> "${root}/debug.log"
cd -
cat /tmp/7zextra/7zS.sfx "${atom}/windows_amd64/extractor_config.txt" "/tmp/${version}_windows_amd64.7z" > "/tmp/${version}_windows_amd64.exe"
rm -rf "${atom}/windows_amd64/"*
cp -f "/tmp/${version}_windows_amd64.exe" "${atom}/windows_amd64/bowery.exe"
cp -f "${datadir}/README_windows" "${atom}/windows_amd64/README"

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
"${root}/scripts/util" aws "${bucket}" "${distdir}"
