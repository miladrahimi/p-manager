.PHONY: dev-setup dev-run setup recover clean fresh update

dev-setup:
	@./scripts/dev-setup.sh

dev-run:
	@go run main.go start

setup:
	@git pull
	@./scripts/setup.sh

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
