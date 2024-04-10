#!/bin/bash

# Setup Configuration
ROOT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )/.."
if [ -f $ROOT_DIR/configs/main.local.json ]; then
		mv $ROOT_DIR/configs/main.local.json $ROOT_DIR/configs/main.json;
fi

docker compose pull
docker compose down
rm -f ./storage/logs/*.log
docker compose up -d
date '+%Y-%m-%d %H:%M:%S' > ./storage/app/update.txt
