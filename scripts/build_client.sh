#!/usr/bin/env bash
set -e

root="$(cd -P "$(dirname "${BASH_SOURCE[0]}")" && pwd)/../"
bowery_app="${root}/BoweryMenubarApp/Bowery/Bowery"
mkdir -p "${bowery_app}"
cd "${root}/bowery/client"

echo "--> Building Client..."
go build -o bin/client
cp -R bin/client public templates "${bowery_app}"
