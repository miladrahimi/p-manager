#!/bin/bash

rm -f ./storage/logs/*.log
date '+%Y-%m-%d %H:%M:%S Started' > ./storage/app/update.txt

if type docker >/dev/null 2>&1; then
    docker compose down
fi

ROOT=$(realpath "$(dirname "${BASH_SOURCE[0]}")/..")
SERVICE_NAME=$(basename "$ROOT")
SETUP_SCRIPT="$(dirname "$0")/setup.sh"

service_exists() {
    systemctl list-units --full -all | grep -Fq "$SERVICE_NAME.service"
}

service_active() {
    systemctl is-active --quiet "$SERVICE_NAME"
}

if service_exists; then
    if service_active; then
        systemctl restart "$SERVICE_NAME"
    else
        systemctl start "$SERVICE_NAME"
    fi
else
    $SETUP_SCRIPT
fi

date '+%Y-%m-%d %H:%M:%S Done' >> ./storage/app/update.txt
