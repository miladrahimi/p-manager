#!/bin/bash

docker compose pull
docker compose down
rm -f ./storage/logs/*.log
docker compose up -d
date '+%Y-%m-%d %H:%M:%S' > ./storage/app/update.txt
