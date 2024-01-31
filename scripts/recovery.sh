#!/bin/bash

script_dir=$(dirname "$(realpath "$0")")
latest_backup=$(ls -t "$script_dir/../storage/backup-"* 2>/dev/null | head -n 1)
if [ -n "$latest_backup" ]; then
  cp "$latest_backup" "$script_dir/../storage/database.json"
  echo "$latest_backup recovered successfully."
else
    echo "No backup files found."
fi
