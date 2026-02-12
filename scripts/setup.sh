#!/bin/bash

# Setup script to install required packages and configure the node.

# Check if running as root
if [ "$(id -u)" -ne 0 ]; then
    echo "This script must be run as root."
    exit 1
fi

REQUIRED_PACKAGES=("make" "wget" "curl" "jq" "vim" "git" "openssl" "cron")

# Update OS repositories if needed
UPDATE_NEEDED=false
for PACKAGE in "${REQUIRED_PACKAGES[@]}"; do
    if ! dpkg -l | grep -q "^ii  $PACKAGE "; then
        UPDATE_NEEDED=true
        break
    fi
done
if [ "$UPDATE_NEEDED" = true ]; then
    echo "Some packages need to be installed. Updating package lists..."
    apt-get -y update
fi

# Install required packages if they're not already installed
for PACKAGE in "${REQUIRED_PACKAGES[@]}"; do
    if ! dpkg -l | grep -q "^ii  $PACKAGE "; then
        echo "Installing $PACKAGE..."
        apt-get -y install "$PACKAGE"
    fi
done

# Detect basic variables
ROOT=$(realpath "$(dirname "${BASH_SOURCE[0]}")/..")
SERVICE_NAME=$(basename "$ROOT")

# Configure Git
git config pull.rebase false

# Configure storage permissions
chmod 0777 "$ROOT/storage"

# Validate the binary file
BINARY_PATH="$ROOT/p-manager"
if [ ! -f "$BINARY_PATH" ]; then
    echo "Binary not found at $BINARY_PATH"
    exit 1
fi

# Create the config file if it doesn't exist
if [ ! -f "$ROOT"/configs/main.json ]; then
    cp "$ROOT"/configs/main.example.json "$ROOT"/configs/main.json
fi

# Generate the service file from the template
SERVICE_FILE="/etc/systemd/system/$SERVICE_NAME.service"
SERVICE_TEMPLATE="$ROOT/scripts/service.template"
generate_service_file() {
    sed "s|THE_NAME|$SERVICE_NAME|" "$SERVICE_TEMPLATE" > "$SERVICE_FILE"
    sed -i "s|THE_PATH|$BINARY_PATH|" "$SERVICE_FILE"
    sed -i "s|THE_DIR|$ROOT|" "$SERVICE_FILE"
}

# Check if service already exists
SERVICE_EXISTS=false
if [ -f "$SERVICE_FILE" ]; then
    SERVICE_EXISTS=true
elif systemctl list-unit-files --type=service --all | awk '{print $1}' | grep -qx "${SERVICE_NAME}.service"; then
    SERVICE_EXISTS=true
fi

# Create or update the service file, reload systemd, enable and restart the service
if [ "$SERVICE_EXISTS" = true ]; then
    echo "Service $SERVICE_NAME already exists. Updating unit file..."
    generate_service_file
    systemctl daemon-reload

    if ! systemctl is-enabled --quiet "$SERVICE_NAME"; then
        echo "Enabling service $SERVICE_NAME..."
        systemctl enable "$SERVICE_NAME"
        echo "Service $SERVICE_NAME enabled."
    fi

    echo "Restarting service $SERVICE_NAME..."
    systemctl restart "$SERVICE_NAME"
    echo "Service $SERVICE_NAME restarted."
else
    echo "Service $SERVICE_NAME not found. Installing unit file..."
    generate_service_file
    systemctl daemon-reload
    echo "Enabling service $SERVICE_NAME..."
    systemctl enable "$SERVICE_NAME"
    echo "Service $SERVICE_NAME enabled."
    echo "Starting service $SERVICE_NAME..."
    systemctl start "$SERVICE_NAME"
    echo "Service $SERVICE_NAME started."
fi

# Setup Cron Job for updating the node
COMMAND="make -C $ROOT update"
if ! crontab -l | grep -q "$COMMAND"; then
    (crontab -l 2>/dev/null; echo "0 4 * * * $COMMAND") | crontab -
    echo "The updater cron job configured."
fi

# Store update time
rm -f "$ROOT"/storage/logs/*.log
date '+%Y-%m-%d %H:%M:%S' > "$ROOT"/storage/app/update.txt
