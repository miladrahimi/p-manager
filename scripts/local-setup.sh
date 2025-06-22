#!/bin/bash

# Detect basic variables
ROOT=$(realpath "$(dirname "${BASH_SOURCE[0]}")/..")

# Install Xray for Mac
THIRD_PARTY="$ROOT/third_party"
DIRECTORY="${THIRD_PARTY}/xray-macos-arm64"
ZIP_FILE="${DIRECTORY}.zip"
rm -rf "$DIRECTORY";
mkdir -p "$DIRECTORY"
wget -qNc https://github.com/XTLS/Xray-core/releases/download/v25.6.8/Xray-macos-arm64-v8a.zip -O "$ZIP_FILE"
unzip "$ZIP_FILE" -d "$DIRECTORY"
echo "${ZIP_FILE}"
rm -rf "${ZIP_FILE}"
