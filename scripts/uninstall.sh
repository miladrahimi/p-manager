#!/bin/bash

# Uninstall script to remove the systemd service, cron job and the directory.

# Check if running as root
if [ "$(id -u)" -ne 0 ]; then
    echo "This script must be run as root."
    exit 1
fi

# Detect basic variables
ROOT=$(realpath "$(dirname "${BASH_SOURCE[0]}")/..")
SERVICE_NAME=$(basename "$ROOT")
BINARY_PATH="$ROOT/p-manager"

resolve_service_name() {
    local name="$SERVICE_NAME"
    local file="/etc/systemd/system/$SERVICE_NAME.service"

    if [ -f "$file" ]; then
        printf "%s" "$name"
        return
    fi

    if [ -d /etc/systemd/system ]; then
        local match
        match=$(grep -rl "ExecStart=$BINARY_PATH" /etc/systemd/system/*.service 2>/dev/null | head -n1)
        if [ -z "$match" ]; then
            match=$(grep -rl "WorkingDirectory=$ROOT" /etc/systemd/system/*.service 2>/dev/null | head -n1)
        fi
        if [ -n "$match" ]; then
            printf "%s" "$(basename "$match" .service)"
            return
        fi
    fi

    printf "%s" "$name"
}

SERVICE_NAME=$(resolve_service_name)
SERVICE_FILE="/etc/systemd/system/$SERVICE_NAME.service"

# Stop and disable the service if it exists
if systemctl list-unit-files --type=service --all | awk '{print $1}' | grep -qx "${SERVICE_NAME}.service"; then
    if systemctl is-active --quiet "$SERVICE_NAME"; then
        echo "Stopping service $SERVICE_NAME..."
        systemctl stop "$SERVICE_NAME"
    fi

    if systemctl is-enabled --quiet "$SERVICE_NAME"; then
        echo "Disabling service $SERVICE_NAME..."
        systemctl disable "$SERVICE_NAME"
    fi

    if systemctl is-failed --quiet "$SERVICE_NAME"; then
        echo "Resetting failed state for $SERVICE_NAME..."
        systemctl reset-failed "$SERVICE_NAME"
    fi
else
    echo "Service $SERVICE_NAME not found."
fi

# Remove the service unit file
if [ -f "$SERVICE_FILE" ]; then
    echo "Removing unit file $SERVICE_FILE..."
    rm -f "$SERVICE_FILE"
    systemctl daemon-reload
    echo "Unit file removed."
fi

# Remove the updater cron job
COMMAND="make -C $ROOT update"
if crontab -l 2>/dev/null | grep -q "$COMMAND"; then
    crontab -l 2>/dev/null | grep -v "$COMMAND" | crontab -
    echo "The updater cron job removed."
else
    echo "The updater cron job was not found."
fi

# Remove the application directory
if [ -d "$ROOT" ]; then
    echo "Removing application directory $ROOT..."
    cd / || exit 1
    rm -rf "$ROOT"
    echo "Directory removed."
fi
