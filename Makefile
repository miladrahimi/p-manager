.PHONY: local-setup
local-setup:
	@./scripts/local-setup.sh

.PHONY: local-serve
local-serve:
	@go run main.go serve

.PHONY: local-clean
local-clean:
	@rm -f storage/logs/*.log

.PHONY: local-fresh
local-fresh:
	@rm -f storage/app/*.txt
	@rm -f storage/app/*.json
	@rm -f storage/database/*.json
	@rm -f storage/logs/*.log

.PHONY: build
build:
	@GOOS=linux GOARCH=amd64 go build -o p-manager

.PHONY: setup
setup:
	@./scripts/setup.sh

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
	@git fetch --all
	@git reset --hard
	@git clean -fd
	@git pull
	@./scripts/setup.sh

.PHONY: schedule-server-reboot
schedule-server-reboot:
	@./scripts/schedule-server-reboot.sh
