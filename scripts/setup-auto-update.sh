#!/bin/bash

# Get the directory of the Makefile
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )/.."

# Define the command to run
COMMAND="make -C $SCRIPT_DIR update"

# Add the cron job to the user's crontab
(crontab -l 2>/dev/null; echo "*/5 * * * * $COMMAND") | crontab -

echo "The cron job for auto-update is configured to be run every night at 04:00 AM."
