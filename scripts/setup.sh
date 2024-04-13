#!/bin/bash

ROOT="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )/.."

# Configure Git
git config pull.rebase false

# Create Config File
if [ ! -f "$ROOT"/configs/main.json ]; then
		cp "$ROOT"/configs/main.defaults.json "$ROOT"/configs/main.json;
fi

# Setup Cron Jobs
COMMAND="make -C $ROOT update"
if ! crontab -l | grep -q "$COMMAND"; then
    (crontab -l 2>/dev/null; echo "0 4 * * * $COMMAND") | crontab -
    echo "The updater cron job configured."
fi
