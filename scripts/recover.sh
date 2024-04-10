#!/bin/bash

docker compose down

ROOT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )/.."
LATEST_BACKUP=$(ls -t "$ROOT_DIR/storage/database/backup-"* 2>/dev/null | head -n 1)
if [ -n "$LATEST_BACKUP" ]; then
  cp "$LATEST_BACKUP" "$ROOT_DIR/storage/database/app.json"
  echo "$LATEST_BACKUP recovered successfully."
else
    echo "No backup files found."
fi

docker compose up -d
