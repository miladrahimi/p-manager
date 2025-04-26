.PHONY: local-setup
local-setup:
	@./scripts/local-setup.sh

.PHONY: local-run
local-run:
	@go run main.go start

.PHONY: build
build:
	@GOOS=linux GOARCH=amd64 go build -o p-manager

.PHONY: setup
setup:
	@./scripts/setup.sh

.PHONY: schedule-reboot
schedule-reboot:
	@./scripts/schedule-reboot.sh

.PHONY: recover
recover:
	@./scripts/recover.sh

.PHONY: clean
clean:
	@rm -f storage/logs/*.log

.PHONY: fresh
fresh:
	@rm -f storage/app/*.txt
	@rm -f storage/app/*.json
	@rm -f storage/database/*.json
	@rm -f storage/logs/*.log

.PHONY: update
update:
	@git reset --hard HEAD^
	@git pull
	@./scripts/setup.sh
	@./scripts/update.sh
