# Makefile for gemini-cli-go

.PHONY: help install build test lint format clean start run

BINARY_NAME=gemini-cli

help:
	@echo "Makefile for gemini-cli-go"
	@echo ""
	@echo "Usage:"
	@echo "  make install          - Install Go dependencies"
	@echo "  make build            - Build the Go project"
	@echo "  make test             - Run the test suite"
	@echo "  make lint             - Lint the code"
	@echo "  make format           - Format the code"
	@echo "  make clean            - Remove generated files"
	@echo "  make start            - Run the Gemini CLI"
	@echo "  make run              - Run the Gemini CLI (alias for start)"


install:
	go mod tidy

build:
	go build -o $(BINARY_NAME) .

test:
	go test ./...

lint:
	go vet ./...

format:
	go fmt ./...

clean:
	rm -f $(BINARY_NAME)

start:
	go run main.go

run: start