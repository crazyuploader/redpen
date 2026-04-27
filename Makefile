BIN      := redpen
CMD      := ./cmd/redpen
VERSION  ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS  := -ldflags "-s -w -X main.version=$(VERSION)"

.PHONY: build test lint clean install

build:
	go build $(LDFLAGS) -o $(BIN) $(CMD)

install:
	go install $(LDFLAGS) $(CMD)

test:
	go test -race ./...

lint:
	golangci-lint run ./...

clean:
	rm -f $(BIN)

.DEFAULT_GOAL := build
