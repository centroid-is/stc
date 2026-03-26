.PHONY: build test lint install test-full clean

VERSION ?= dev
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
DATE    ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS := -X github.com/centroid-is/stc/pkg/version.Version=$(VERSION) \
           -X github.com/centroid-is/stc/pkg/version.Commit=$(COMMIT) \
           -X github.com/centroid-is/stc/pkg/version.Date=$(DATE)

build:
	go build ./...
	go build -ldflags "$(LDFLAGS)" -o stc ./cmd/stc

test:
	go test ./... -count=1

lint:
	golangci-lint run ./...

install:
	go install -ldflags "$(LDFLAGS)" ./cmd/stc

test-full:
	go test ./... -v -race -count=1 -coverprofile=coverage.out

clean:
	rm -f stc coverage.out
	go clean ./...
