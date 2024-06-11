#!/bin/bash

docker compose down

ROOT=$(realpath "$(dirname "${BASH_SOURCE[0]}")/..")
LAST_BACKUP=$(ls -t "$ROOT/storage/database/backup-"* 2>/dev/null | head -n 1)
if [ -n "$LAST_BACKUP" ]; then
  cp "$LAST_BACKUP" "$ROOT/storage/database/app.json"
  echo "$LAST_BACKUP recovered successfully."
else
    echo "No backup file found."
fi

docker compose up -d
