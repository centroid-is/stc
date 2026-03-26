.PHONY: build test lint install test-full clean

build:
	go build ./...

test:
	go test ./... -count=1

lint:
	golangci-lint run ./...

install:
	go install ./cmd/stc

test-full:
	go test ./... -v -race -count=1 -coverprofile=coverage.out

clean:
	rm -f stc coverage.out
	go clean ./...
