.PHONY: dev_setup dev_run dev_fresh dev_clean setup recover fresh update

dev_setup:
	@./scripts/dev_setup.sh

dev_run:
	@go run main.go start

dev_fresh:
	@rm -f storage/app/*.txt
	@rm -f storage/app/*.json
	@rm -f storage/database/*.json
	@rm -f storage/logs/*.log

dev_clean:
	@rm -f storage/logs/*.log

setup:
	@./scripts/setup.sh

recover:
	@./scripts/recover.sh

fresh:
	@rm -f storage/app/*.txt
	@rm -f storage/app/*.json
	@rm -f storage/database/*.json
	@rm -f storage/logs/*.log
	@docker compose restart

update: setup
	@git pull
	@./scripts/update.sh
