## Makefile for mesa-ads
.SILENT:
.PHONY:

all: generate lint test build

generate:
	go generate -trimpath ./...

lint:
	golangci-lint run --timeout=5m

fix:
	golangci-lint run --timeout=5m --fix

test:
	go test -trimpath -coverprofile coverage.txt ./...
	go tool cover -html coverage.txt -o coverage.html
	printf "total coverage: "
	go tool cover -func=coverage.txt | grep total | grep -oE '[0-9]+\.[0-9]+%'

build:
	go build -trimpath -o mesa-ads mesa-ads/cmd

clean:
	rm -rf coverage.* mesa-ads

install:
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.64.8
	go install github.com/vektra/mockery/v3@v3.2.5