#!/bin/bash

# Check if running as root
if [ "$(id -u)" -ne 0 ]; then
    echo "This script must be run as root"
    exit 1
fi

# Detect basic variables
ROOT=$(realpath "$(dirname "${BASH_SOURCE[0]}")/..")
SERVICE_NAME=$(basename "$ROOT")

# Stop the service
systemctl stop "$SERVICE_NAME"

# Replace database with the last backup
LAST_BACKUP=$(ls -t "$ROOT/storage/database/backup-"* 2>/dev/null | head -n 1)
if [ -n "$LAST_BACKUP" ]; then
  cp "$LAST_BACKUP" "$ROOT/storage/database/app.json"
  echo "$LAST_BACKUP recovered successfully."
else
    echo "No backup file found."
fi

# Start the service
systemctl start "$SERVICE_NAME"
