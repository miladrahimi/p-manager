#!/bin/bash

ROOT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )/.."

COMMAND="make -C $ROOT_DIR update"

if ! crontab -l | grep -q "$COMMAND"; then
    (crontab -l 2>/dev/null; echo "0 4 * * * $COMMAND") | crontab -
    echo "The updater cron job configured."
else
    echo "The updater cron job already exists."
fi
