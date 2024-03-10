#!/bin/bash

BASE="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )/../third_party"
DIR="${BASE}/xray-macos-arm64"
FILE="${DIR}.zip"
rm -rf "$DIR";
mkdir -p "$DIR"
wget -qNc https://github.com/XTLS/Xray-core/releases/download/v1.8.8/Xray-macos-arm64-v8a.zip -O "$FILE"
unzip "$FILE" -d "$DIR"
echo "${FILE}"
rm -rf "${FILE}"
