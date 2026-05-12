.PHONY: build test lint run clean coverage tidy

BIN := bin/mkdev
PKG := ./...
GOFLAGS := -trimpath

VERSION := $(shell git describe --tags --dirty --always 2>/dev/null || echo "dev")
COMMIT  := $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
DATE    := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)

LDFLAGS := -X github.com/venkatkrishna07/mkdev/internal/version.Version=$(VERSION) \
           -X github.com/venkatkrishna07/mkdev/internal/version.Commit=$(COMMIT) \
           -X github.com/venkatkrishna07/mkdev/internal/version.Date=$(DATE)

build:
	@mkdir -p bin
	go build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(BIN) ./cmd/mkdev

test:
	go test $(GOFLAGS) -race -count=1 -timeout=60s $(PKG)

lint:
	golangci-lint run

run: build
	$(BIN)

coverage:
	go test $(GOFLAGS) -race -count=1 -coverprofile=coverage.txt -covermode=atomic $(PKG)
	go tool cover -func=coverage.txt | tail -1

tidy:
	go mod tidy
	gofmt -w .

clean:
	rm -rf bin coverage.txt
