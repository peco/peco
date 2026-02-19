MAKEFLAGS += --no-print-directory
export PATH := $(HOME)/go/bin:$(PATH)

.PHONY: build snapshot test lint generate cover clean deps check-goreleaser

check-goreleaser:
	@command -v goreleaser >/dev/null 2>&1 || { echo "goreleaser not found. Install it: https://goreleaser.com/install/"; exit 1; }

build: check-goreleaser
	goreleaser build --snapshot --single-target --clean

snapshot: check-goreleaser
	goreleaser release --snapshot --clean

test:
	go test -v -race ./...

lint:
	golangci-lint run ./...

generate:
	go generate ./...

cover:
	go test -race -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out

clean:
	rm -rf dist

deps:
	go mod download
