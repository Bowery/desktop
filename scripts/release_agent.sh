#!/bin/bash

# Get the parent directory of where this script is.
SOURCE="${BASH_SOURCE[0]}"
while [ -h "$SOURCE" ] ; do SOURCE="$(readlink "$SOURCE")"; done
DIR="$( cd -P "$( dirname "$SOURCE" )/../" && pwd )"
CGO_ENABLED=0

DIR="$DIR/bowery/agent"

echo "here $DIR"
# Change into that dir because we expect that
cd "$DIR"

# Determine the version that we're building based on the contents
# of delancey/VERSION.
VERSION=$(cat ../VERSION)
VERSIONDIR="${VERSION}"
echo "Version: ${VERSION}"

# Make sure that if we're killed, we kill all our subprocseses
# trap "kill 0" SIGINT SIGTERM EXIT

# Make sure goxc is installed
go get -u github.com/laher/goxc

# This function builds whatever directory we're in...
goxc \
    -tasks-="validate" \
    -d="${DIR}/pkg" \
    -pv="${VERSION}" \
    $XC_OPTS \
    go-install \
    xc

# tar+gzip all the packages
mkdir -p "./pkg/${VERSIONDIR}/dist"
for PLATFORM in $(find "./pkg/${VERSIONDIR}" -mindepth 1 -maxdepth 1 -type d); do
    PLATFORM_NAME=$(basename ${PLATFORM})
    ARCHIVE_NAME="${VERSIONDIR}_${PLATFORM_NAME}"

    if [ $PLATFORM_NAME = "dist" ]; then
        continue
    fi

    pushd ${PLATFORM}
    tar -czf "${DIR}/pkg/${VERSIONDIR}/dist/${ARCHIVE_NAME}.tar.gz" ./*
    popd
done

echo $VERSION > "./pkg/${VERSIONDIR}/dist/VERSION"
cp -r "${DIR}/init/" "./pkg/${VERSIONDIR}/dist/"
cp "../../scripts/install_agent.sh" "./pkg/${VERSIONDIR}/dist/"

# Make the checksums
pushd "./pkg/${VERSIONDIR}/dist"
shasum -a256 * > "./${VERSIONDIR}_SHA256SUMS"
popd

for ARCHIVE in ./pkg/${VERSION}/dist/*; do
    ARCHIVE_NAME=$(basename ${ARCHIVE})
    echo Uploading: $ARCHIVE_NAME from $ARCHIVE
    file=$ARCHIVE_NAME
    bucket=bowery.sh
    resource="/${bucket}/${file}"
    contentType="application/octet-stream"
    dateValue=`date -u +"%a, %d %h %Y %T +0000"`
    stringToSign="PUT\n\n${contentType}\n${dateValue}\n${resource}"
    s3Key=AKIAI6ICZKWF5DYYTETA
    s3Secret=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
    signature=`echo -en ${stringToSign} | openssl sha1 -hmac ${s3Secret} -binary | base64`
    curl -k\
        -T ${ARCHIVE} \
        -H "Host: ${bucket}.s3.amazonaws.com" \
        -H "Date: ${dateValue}" \
        -H "Content-Type: ${contentType}" \
        -H "Authorization: AWS ${s3Key}:${signature}" \
        https://${bucket}.s3.amazonaws.com/${file}
done
