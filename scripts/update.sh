git pull
docker compose pull
docker compose down
rm ./storage/logs/*.log
docker compose up -d
echo "$(date '+%Y-%m-%d %H:%M:%S') Updated." >> ./storage/app/updates.txt
