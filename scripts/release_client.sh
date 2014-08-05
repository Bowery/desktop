#!/usr/bin/env bash
set -e

# Make sure that if we're killed, we kill all our subprocseses
trap "kill 0" SIGINT SIGTERM EXIT

root="$(cd -P "$(dirname "${BASH_SOURCE[0]}")" && pwd)/../"
version="$(cat "${root}/bowery/VERSION")"
mkdir -p "${root}/BoweryMenubarApp/pkg/${version}/dist"
cd "${root}"
echo "Version: ${version}"

# Build the main app.
./scripts/build_client.sh
cd BoweryMenubarApp
xcodebuild -target Bowery

# Compress the app.
cd build/Release
tar -czf "../../pkg/${version}/dist/${version}.tar.gz" Bowery.app
cd -
cp "pkg/${version}/dist/${version}.tar.gz" "pkg/${version}/dist/latest.tar.gz"

# Write release support files.
echo "${version}" > "pkg/${version}/dist/VERSION"
cd "pkg/${version}/dist"
shasum -a256 * > "${version}_SHA256SUMS"
cd -

for path in "pkg/${version}/dist/"*; do
  file="$(basename "${path}")"
  echo "Uploading: ${file} from ${path}"
  bucket=mac.bowery.io
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
