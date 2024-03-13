docker compose pull
docker compose down
rm -f ./storage/logs/*.log
rm -f ./storage/*.*
docker compose up -d
date '+%Y-%m-%d %H:%M:%S' >> ./storage/app/updates.txt
