#!/bin/bash

script_dir=$(dirname "$(realpath "$0")")
latest_backup=$(ls -t "$script_dir/../storage/database/backup-"* 2>/dev/null | head -n 1)
if [ -n "$latest_backup" ]; then
  cp "$latest_backup" "$script_dir/../storage/database/app.json"
  echo "$latest_backup recovered successfully."
else
    echo "No backup files found."
fi
