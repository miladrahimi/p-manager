.PHONY: setup run build reset fresh update

setup:
	./third_party/install-xray-mac.sh

run:
	go run main.go start

build:
	go build main.go -o xray-manager

fresh:
	rm storage/database.json
	rm storage/xray.json
	docker compose restart

update:
	git pull
	docker compose pull
	docker compose down
	docker compose up -d
