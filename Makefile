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

TAILWIND_VERSION := v4.3.3

bin/tailwindcss:
	@mkdir -p bin
	@curl -sL -o bin/tailwindcss "https://github.com/tailwindlabs/tailwindcss/releases/download/$(TAILWIND_VERSION)/tailwindcss-$$(uname -s | sed 's/Darwin/macos/;s/Linux/linux/')-$$(uname -m | sed 's/x86_64/x64/;s/aarch64/arm64/')"
	@chmod +x bin/tailwindcss

# Rebuild the admin UI stylesheet (web/assets/css/app.css, committed).
.PHONY: web
web: bin/tailwindcss
	@./bin/tailwindcss -i web/assets/css/app.src.css -o web/assets/css/app.css --minify

.PHONY: setup
setup:
	@./scripts/setup.sh

.PHONY: recover
recover:
	@./scripts/recover.sh

.PHONY: uninstall
uninstall:
	@./scripts/uninstall.sh

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
