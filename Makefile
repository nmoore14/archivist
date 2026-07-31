.PHONY: help setup build demo verify status logs stop test

help:
	@echo "Archivist setup and operations"
	@echo
	@echo "  make setup   Guided installation menu"
	@echo "  make build   Build the standard Archivist containers"
	@echo "  make demo    Build and start the portable demo"
	@echo "  make verify  Verify an installed Linux deployment is offline"
	@echo "  make status  Show container status"
	@echo "  make logs    Follow container logs"
	@echo "  make stop    Stop the standard local containers"
	@echo "  make test    Run the application tests"

setup:
	@./deploy/linux/guided-setup.sh

build:
	docker compose build

demo:
	docker compose -f deploy/demo-parent/compose.yml up --build -d --wait
	./deploy/demo-parent/verify.sh

verify:
	@sudo ./deploy/linux/verify.sh

status:
	@docker compose ps

logs:
	@docker compose logs -f

stop:
	@docker compose down

test:
	go test ./...
