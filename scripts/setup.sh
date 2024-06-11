#!/bin/bash

# Check if running as root
if [ "$(id -u)" -ne 0 ]; then
    echo "This script must be run as root"
    exit 1
fi

# Detect root directory
ROOT="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )/.."

# Configure Git
git config pull.rebase false

# Validate the binary exists
BINARY_PATH="$ROOT/p-manager"
if [ ! -f "$BINARY_PATH" ]; then
    echo "Binary not found at $BINARY_PATH"
    exit 1
fi

# Setup service
SERVICE_NAME="p-manager"
SERVICE_FILE="/etc/systemd/system/$SERVICE_NAME.service"

SERVICE_TEMPLATE="$ROOT/scripts/service.template"
sed "s|/path/to/binary|$BINARY_PATH|" "$SERVICE_TEMPLATE" > $SERVICE_FILE

if systemctl is-enabled --quiet $SERVICE_NAME; then
    echo "$SERVICE_NAME service is already enabled and installed."
else
    systemctl daemon-reload
    systemctl enable $SERVICE_NAME
    systemctl start $SERVICE_NAME
    echo "$SERVICE_NAME service started."
fi

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
