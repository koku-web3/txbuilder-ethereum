.PHONY: build test fmt lint 

all: build test fmt lint

build:
	go build ./...

test:
	go test ./...

fmt:
	gofmt -w .

lint:
	golangci-lint run ./...