#!/bin/bash

# Install requirements for developing the project on local (macOS) machine.

# Detect basic variables
ROOT=$(realpath "$(dirname "${BASH_SOURCE[0]}")/..")
THIRD_PARTY="$ROOT/third_party"

# Install Xray
DIRECTORY="${THIRD_PARTY}/xray-macos-arm64"
ZIP_FILE="${DIRECTORY}.zip"
rm -rf "$DIRECTORY";
mkdir -p "$DIRECTORY"
wget -qNc https://github.com/XTLS/Xray-core/releases/download/v26.3.27/Xray-macos-arm64-v8a.zip -O "$ZIP_FILE"
unzip "$ZIP_FILE" -d "$DIRECTORY"
echo "${ZIP_FILE}"
rm -rf "${ZIP_FILE}"

# Install paqet (Relay RR2PQ; needs root to run, see README)
DIRECTORY="${THIRD_PARTY}/paqet-macos-arm64"
TAR_FILE="${DIRECTORY}.tar.gz"
rm -rf "$DIRECTORY";
mkdir -p "$DIRECTORY"
wget -qNc https://github.com/hanselime/paqet/releases/download/v1.0.0-alpha.21/paqet-darwin-arm64-v1.0.0-alpha.21.tar.gz -O "$TAR_FILE"
tar xzf "$TAR_FILE" -C "$DIRECTORY"
mv "$DIRECTORY"/paqet_darwin_arm64 "$DIRECTORY/paqet"
echo "${TAR_FILE}"
rm -rf "${TAR_FILE}"
