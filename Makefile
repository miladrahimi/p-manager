.PHONY: setup run build reset fresh update

setup:
	./third_party/install-xray-mac.sh

run:
	go run main.go start

build:
	go build main.go -o xray-manager

recover:
	docker compose down
	./scripts/recovery.sh
	docker compose up -d

fresh:
	rm storage/*.json
	docker compose restart

update:
	git pull
	docker compose pull
	docker compose down
	rm ./storage/xray.json
	docker compose up -d
