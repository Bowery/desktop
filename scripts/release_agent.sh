#!/bin/bash

# Get the full path to the parent of this script.
source="${BASH_SOURCE[0]}"
while [[ -h "${source}" ]]; do source="$(readlink "${source}")"; done
root="$(cd -P "$(dirname "${source}")/.." && pwd)"
CGO_ENABLED=0

agent="${root}/agent"
updater="${root}/updater"
bucket=bowery.sh
s3endpoint="https://${bucket}.s3.amazonaws.com"

echo "Agent dir ${agent}"
cd "${agent}"

# Get the version we're building.
version="$(cat VERSION)"
echo "Version: ${version}"

cd "${root}/scripts"
go build util.go
cd -

go get -u github.com/laher/goxc

# Build the agent.
goxc \
  -wd="${agent}" \
  -d="${agent}/pkg" \
  -pv="${version}" \
  ${XC_OPTS} \
  xc

# Build the updater, output to agent/pkg.
goxc \
  -wd="${updater}" \
  -d="${agent}/pkg" \
  -pv="${version}" \
  ${XC_OPTS} \
  xc

# Tar+gzip up the binaries and add download urls to the VERSION file.
mkdir -p "pkg/${version}/dist"
echo "${version}" > "pkg/${version}/dist/VERSION"
for platform in $(find "pkg/${version}" -mindepth 1 -maxdepth 1 -type d); do
  platform_name="$(basename "${platform}")"
  archive="${version}_${platform_name}.tar.gz"

  if [[ "${platform_name}" == "dist" ]]; then
    continue
  fi

  pushd "${platform}"
  mv agent bowery-agent
  mv updater bowery-updater
  mv agent.exe bowery-agent.exe 2>/dev/null
  mv updater.exe bowery-updater.exe 2>/dev/null
  tar -czf "${agent}/pkg/${version}/dist/${archive}" *
  echo "${s3endpoint}/${archive}" >> "${agent}/pkg/${version}/dist/VERSION"
  popd
done

# Copy support files and checksums
cp -r "${agent}/init/"* "pkg/${version}/dist"
cp "${root}/scripts/install_agent.sh" "pkg/${version}/dist"
pushd "pkg/${version}/dist"
shasum -a256 * > "${version}_SHA256SUMS"
popd

"${root}/scripts/util" aws "${bucket}" "pkg/${version}/dist"
