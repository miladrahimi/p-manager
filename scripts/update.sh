echo "$(shell date '+%Y-%m-%d %H:%M:%S') Updating..." >> ./storage/app/updates.txt
git pull
docker compose pull
docker compose down
rm ./storage/logs/*.log
docker compose up -d
echo "$(shell date '+%Y-%m-%d %H:%M:%S') Updated." >> ./storage/app/updates.txt
