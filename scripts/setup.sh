#!/bin/bash

ROOT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )/.."

# Setup Git
git config pull.rebase false

# Setup Configuration
if [ -f $ROOT_DIR/configs/main.local.json ]; then
		mv $ROOT_DIR/configs/main.local.json $ROOT_DIR/configs/main.json;
fi
if [ ! -f $ROOT_DIR/configs/main.json ]; then
		cp $ROOT_DIR/configs/main.defaults.json $ROOT_DIR/configs/main.json;
fi

# Setup Cron Jobs
COMMAND="make -C $ROOT_DIR update"
if ! crontab -l | grep -q "$COMMAND"; then
    (crontab -l 2>/dev/null; echo "0 4 * * * $COMMAND") | crontab -
    echo "The updater cron job configured."
else
    echo "The updater cron job already exists."
fi
