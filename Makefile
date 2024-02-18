.PHONY: setup run build setup-auto-update recover fresh update

setup:
	./third_party/install-xray-mac.sh

run:
	go run main.go start

build:
	go build main.go -o xray-manager

setup-auto-update:
	./scripts/setup-auto-update.sh

recover:
	docker compose down
	./scripts/recovery.sh
	docker compose up -d

fresh:
	rm storage/*.json
	docker compose restart

update:
	@echo "$(shell date '+%Y-%m-%d %H:%M:%S') Updating..." >> ./storage/updates.txt
	git pull
	docker compose pull
	docker compose down
	rm ./storage/xray.json
	docker compose up -d
	@echo "$(shell date '+%Y-%m-%d %H:%M:%S') Updated." >> ./storage/updates.txt

