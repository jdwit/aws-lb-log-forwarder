.PHONY: build test lint e2e e2e-up e2e-down clean

# Build the binary
build:
	go build -o aws-lb-log-forwarder .

# Run unit tests
test:
	go test -v -race -coverprofile=coverage.out ./...

# Run linter
lint:
	golangci-lint run

# Start e2e test infrastructure
e2e-up:
	docker compose -f e2e/docker-compose.yml up -d
	@echo "Waiting for services to be healthy..."
	@sleep 10

# Stop e2e test infrastructure
e2e-down:
	docker compose -f e2e/docker-compose.yml down -v

# Run e2e tests (requires e2e-up first)
e2e:
	./e2e/run.sh

# Run full e2e cycle
e2e-full: e2e-up e2e e2e-down

# Clean build artifacts
clean:
	rm -f aws-lb-log-forwarder coverage.out
	rm -f e2e/aws-lb-log-forwarder
