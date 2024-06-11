#!/bin/bash

# Check if running as root
if [ "$(id -u)" -ne 0 ]; then
    echo "This script must be run as root"
    exit 1
fi

# Store update time
rm -f ./storage/logs/*.log
date '+%Y-%m-%d %H:%M:%S' > ./storage/app/update.txt

# TODO: Delete this
ROOT=$(realpath "$(dirname "${BASH_SOURCE[0]}")/..")
SERVICE_NAME=$(basename "$ROOT")
if type docker >/dev/null 2>&1; then
    docker compose down --remove-orphans
fi
service_exists() {
    systemctl list-units --full -all | grep -Fq "$SERVICE_NAME.service"
}
if ! service_exists; then
    echo "Running setup again..."
    "$(dirname "$0")/setup.sh"
fi
