docker compose pull
docker compose down
rm -f ./storage/logs/*.log
rm -f ./storage/app/updates.txt
docker compose up -d
date '+%Y-%m-%d %H:%M:%S' > ./storage/app/update.txt
