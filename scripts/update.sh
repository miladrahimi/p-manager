#!/bin/bash

# Handle inconsistencies between new and old versions.

# Detect basic variables
ROOT=$(realpath "$(dirname "${BASH_SOURCE[0]}")/..")

# v26.2.8

if [ ! -f "$ROOT/storage/database/data.json" ] && [ -f "$ROOT/storage/database/app.json" ]; then
  cp -f "$ROOT/storage/database/app.json" "$ROOT/storage/database/data.json"
fi

rm -f "$ROOT/storage/database/app.json"
