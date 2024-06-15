.PHONY: dev-setup dev-run build setup recover clean fresh update

dev-setup:
	@./scripts/dev-setup.sh

dev-run:
	@go run main.go start

build:
	@GOOS=linux GOARCH=amd64 go build -o p-manager

setup:
	@git pull
	@./scripts/setup.sh

schedule-reboot:
	@./scripts/schedule-reboot.sh

recover:
	@./scripts/recover.sh

clean:
	@rm -f storage/logs/*.log

fresh:
	@rm -f storage/app/*.txt
	@rm -f storage/app/*.json
	@rm -f storage/database/*.json
	@rm -f storage/logs/*.log

update: setup
	@./scripts/update.sh
