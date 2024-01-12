.PHONY: install run build reset fresh

install:
	./third_party/install-xray.sh

run: install
	go run main.go start

build: install
	go build main.go -o ssm

fresh:
	rm storage/database.json
	rm storage/xray.json
	docker compose restart
