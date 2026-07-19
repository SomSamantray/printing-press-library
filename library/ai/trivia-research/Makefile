.PHONY: build test lint install clean

build:
	go build -o bin/trivia-research-pp-cli ./cmd/trivia-research-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/trivia-research-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/trivia-research-pp-mcp ./cmd/trivia-research-pp-mcp

install-mcp:
	go install ./cmd/trivia-research-pp-mcp

build-all: build build-mcp
