#!/bin/bash

ROOT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )/.."

COMMAND="make -C $ROOT_DIR update"

(crontab -l 2>/dev/null; echo "0 4 * * * $COMMAND") | crontab -

echo "The cron job for auto-update is configured to be run every night at 04:00 AM."
