BINARY := bin/server
VERSION ?= $(or $(patsubst v%,%,$(shell git describe --tags --always --dirty 2>/dev/null)),dev)
LDFLAGS := -X main.version=$(VERSION)

.PHONY: run build test lint fmt tidy clean docker

run:
	go run ./cmd/server

build:
	go build -ldflags "$(LDFLAGS)" -o $(BINARY) ./cmd/server

test:
	go test ./... -race -shuffle=on -cover

# golangci-lint runs go vet among its linters; .golangci.yml is the config.
lint:
	golangci-lint run ./...

fmt:
	golangci-lint fmt ./...

tidy:
	go mod tidy

clean:
	rm -rf bin

docker:
	docker build --build-arg VERSION=$(VERSION) -t cerebellum-server:$(VERSION) .
