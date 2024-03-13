docker compose pull
docker compose down
rm -f ./storage/logs/*.log
rm -f ./storage/*.*
docker compose up -d
echo "$(date '+%Y-%m-%d %H:%M:%S') Updated." >> ./storage/app/updates.txt
