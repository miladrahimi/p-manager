.PHONY: prepare setup recover fresh update version

prepare:
	./third_party/install-xray-mac.sh

setup:
	./scripts/setup-updater.sh
	@if [ ! -f ./configs/main.local.json ]; then \
		cp ./configs/main.json ./configs/main.local.json; \
	fi

recover:
	docker compose down
	./scripts/recovery.sh
	docker compose up -d

fresh:
	rm storage/*.json
	docker compose restart

update: setup
	@echo "$(shell date '+%Y-%m-%d %H:%M:%S') Updating..." >> ./storage/updates.txt
	git pull
	docker compose pull
	docker compose down
	rm ./storage/xray.json
	docker compose up -d
	@echo "$(shell date '+%Y-%m-%d %H:%M:%S') Updated." >> ./storage/updates.txt


version:
	docker compose exec app ./xray-manager version
