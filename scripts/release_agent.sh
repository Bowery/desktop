#!/usr/bin/env bash
set -e

# Make sure that if we're killed, we kill all our subprocseses
trap "kill 0" SIGINT SIGTERM EXIT

root="$(cd -P "$(dirname "${BASH_SOURCE[0]}")" && pwd)/../"
version="$(cat "${root}/bowery/VERSION")"
mkdir -p "${root}/bowery/agent/pkg/${version}/dist"
cd "${root}/bowery/agent"
echo "Version: ${version}"

# Setup goxc for compiling.
go get github.com/laher/goxc
XC_ARCH=${XC_ARCH:-"386 amd64 arm"}
XC_OS=${XC_OS:-linux}
echo "Arch: ${XC_ARCH}"
echo "OS: ${XC_OS}"

goxc -arch="${XC_ARCH}" -os="${XC_OS}" -pv="${version}" -d=pkg xc

# Compress the binaries.
for platform in "pkg/${version}/"*; do
    platform="$(basename "${platform}")"
    if [[ "${platform}" == dist ]]; then
      continue
    fi
    archive="${version}_${platform}.tar.gz"

    cd "pkg/${version}/${platform}"
    tar -cvzf "../dist/${archive}" *
    cd -
done

# Write release support files.
echo "${version}" > "pkg/${version}/dist/VERSION"
cp agent.conf install_agent.sh "pkg/${version}/dist"
cd "pkg/${version}/dist"
shasum -a256 * > "${version}_SHA256SUMS"
cd -

for path in "pkg/${version}/dist/"*; do
  file="$(basename "${path}")"
  echo "Uploading: ${file} from ${path}"
  bucket=bowery.sh
  resource="/${bucket}/${file}"
  contentType="application/octet-stream"
  dateValue=`date -u +"%a, %d %h %Y %T +0000"`
  stringToSign="PUT\n\n${contentType}\n${dateValue}\n${resource}"
  s3Key=AKIAI6ICZKWF5DYYTETA
  s3Secret=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
  signature=`echo -en ${stringToSign} | openssl sha1 -hmac ${s3Secret} -binary | base64`
  curl -k \
    -T "${path}" \
    -H "Host: ${bucket}.s3.amazonaws.com" \
    -H "Date: ${dateValue}" \
    -H "Content-Type: ${contentType}" \
    -H "Authorization: AWS ${s3Key}:${signature}" \
     https://${bucket}.s3.amazonaws.com/${file}
  echo
done
