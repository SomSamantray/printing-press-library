.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -o bin/bolna-pp-cli$(BIN_EXT) ./cmd/bolna-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/bolna-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/bolna-pp-mcp$(BIN_EXT) ./cmd/bolna-pp-mcp

install-mcp:
	go install ./cmd/bolna-pp-mcp

build-all: build build-mcp
