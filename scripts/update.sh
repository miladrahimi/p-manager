#!/bin/bash

rm -f ./storage/logs/*.log
date '+%Y-%m-%d %H:%M:%S Started' > ./storage/app/update.txt

if type docker >/dev/null 2>&1; then
    docker compose down
fi

ROOT=$(realpath "$(dirname "${BASH_SOURCE[0]}")/..")
SERVICE_NAME=$(basename "$ROOT")

service_exists() {
    systemctl list-units --full -all | grep -Fq "$SERVICE_NAME.service"
}

service_active() {
    systemctl is-active --quiet "$SERVICE_NAME"
}

if service_exists; then
    if service_active; then
        echo "Restarting service $SERVICE_NAME..."
        systemctl restart "$SERVICE_NAME"
        echo "Service $SERVICE_NAME restarted."
    else
        echo "Starting service $SERVICE_NAME..."
        systemctl start "$SERVICE_NAME"
        echo "Service $SERVICE_NAME started."
    fi
else
    echo "Running setup again..."
    "$(dirname "$0")/setup.sh"
fi

date '+%Y-%m-%d %H:%M:%S Done' >> ./storage/app/update.txt
