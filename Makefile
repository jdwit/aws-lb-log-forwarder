.PHONY: help build test lint e2e e2e-up e2e-down clean

.DEFAULT_GOAL := help

## help: show this help message
help:
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/^## /  /'

## build: build the binary
build:
	go build -o aws-lb-log-forwarder .

## test: run unit tests with coverage
test:
	go test -v -race -coverprofile=coverage.out ./...

## lint: run linter
lint:
	golangci-lint run

## e2e-up: start e2e test infrastructure
e2e-up:
	docker compose -f e2e/docker-compose.yml up -d --wait
	@echo "All services healthy"

## e2e-down: stop e2e test infrastructure
e2e-down:
	docker compose -f e2e/docker-compose.yml down -v

## e2e: run e2e tests (requires e2e-up first)
e2e:
	./e2e/run.sh

## e2e-full: run full e2e cycle (up, test, down)
e2e-full: e2e-up e2e e2e-down

## clean: remove build artifacts
clean:
	rm -f aws-lb-log-forwarder coverage.out
	rm -f e2e/aws-lb-log-forwarder
