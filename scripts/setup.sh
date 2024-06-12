#!/bin/bash

# Check if running as root
if [ "$(id -u)" -ne 0 ]; then
    echo "This script must be run as root"
    exit 1
fi

# Detect basic variables
ROOT=$(realpath "$(dirname "${BASH_SOURCE[0]}")/..")
SERVICE_NAME=$(basename "$ROOT")

# Configure Git
git config pull.rebase false

# Validate the binary file
BINARY_PATH="$ROOT/p-manager"
if [ ! -f "$BINARY_PATH" ]; then
    echo "Binary not found at $BINARY_PATH"
    exit 1
fi

# Create the config file
if [ ! -f "$ROOT"/configs/main.json ]; then
		cp "$ROOT"/configs/main.defaults.json "$ROOT"/configs/main.json;
fi

# Setup Systemd
SERVICE_FILE="/etc/systemd/system/$SERVICE_NAME.service"
SERVICE_TEMPLATE="$ROOT/scripts/service.template"

sed "s|THE_NAME|$SERVICE_NAME|" "$SERVICE_TEMPLATE" > "$SERVICE_FILE"
sed -i "s|THE_PATH|$BINARY_PATH|" "$SERVICE_FILE"
sed -i "s|THE_DIR|$ROOT|" "$SERVICE_FILE"
systemctl daemon-reload

if systemctl is-enabled --quiet "$SERVICE_NAME"; then
    echo "Service $SERVICE_NAME is already enabled and installed."
    echo "Restarting service $SERVICE_NAME..."
    systemctl restart "$SERVICE_NAME"
    echo "Service $SERVICE_NAME restarted."
else
    echo "Enabling service $SERVICE_NAME..."
    systemctl enable "$SERVICE_NAME"
    echo "Service $SERVICE_NAME enabled."
    echo "Starting service $SERVICE_NAME..."
    systemctl start "$SERVICE_NAME"
    echo "Service $SERVICE_NAME started."
fi

# Setup cron jobs
COMMAND="make -C $ROOT update"
if ! crontab -l | grep -q "$COMMAND"; then
    (crontab -l 2>/dev/null; echo "0 4 * * * $COMMAND") | crontab -
    echo "The updater cron job configured."
else
    echo "The updater cron job is already configured."
fi
