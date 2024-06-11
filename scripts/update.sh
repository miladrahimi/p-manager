#!/bin/bash

rm -f ./storage/logs/*.log
date '+%Y-%m-%d %H:%M:%S Started' > ./storage/app/update.txt

if type docker >/dev/null 2>&1; then
    docker compose down --remove-orphans
fi

ROOT=$(realpath "$(dirname "${BASH_SOURCE[0]}")/..")
SERVICE_NAME=$(basename "$ROOT")

service_exists() {
    systemctl list-units --full -all | grep -Fq "$SERVICE_NAME.service"
}

if ! service_exists; then
    echo "Running setup again..."
    "$(dirname "$0")/setup.sh"
fi

date '+%Y-%m-%d %H:%M:%S Done' >> ./storage/app/update.txt
