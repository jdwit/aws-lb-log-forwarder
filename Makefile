.PHONY: build build-lambda test clean fmt lint

# Build the binary
build:
	go build -o alb-log-pipe .

# Build for AWS Lambda
build-lambda:
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o bootstrap .
	zip lambda.zip bootstrap
	rm bootstrap

# Run tests
test:
	go test -v ./...

# Clean build artifacts
clean:
	rm -f alb-log-pipe bootstrap lambda.zip

# Format code
fmt:
	go fmt ./...

# Run linter
lint:
	golangci-lint run
