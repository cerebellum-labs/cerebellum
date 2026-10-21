BINARY := bin/server
# Release tags carry the package prefix (server/v1.2.3), so only those are
# matched and the prefix is cut: 1.2.3 on a release tag, 1.2.3-4-gabc1234
# after it, the commit hash before the first release.
VERSION ?= $(or $(patsubst server/v%,%,$(shell git describe --tags --match 'server/v*' --always --dirty 2>/dev/null)),dev)
LDFLAGS := -X main.version=$(VERSION)

.PHONY: run build test lint fmt tidy clean docker release

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

# Tags the current main as server/v$(V) and pushes the tag; the Release
# workflow does the rest (see RELEASING.md). A final version needs its
# CHANGELOG.md section; a release candidate (1.2.3-rc.1) doesn't. A tag whose
# push failed is deleted: it would block the next attempt and make local
# builds report an unreleased version.
release:
	@echo "$(V)" | grep -Eq '^[0-9]+\.[0-9]+\.[0-9]+(-rc\.[0-9]+)?$$' || { echo "usage: make release V=1.2.3 (or V=1.2.3-rc.1)"; exit 1; }
	@test "$$(git rev-parse --abbrev-ref HEAD)" = main || { echo "release from main"; exit 1; }
	@test -z "$$(git status --porcelain)" || { echo "commit or stash your changes first"; exit 1; }
	@git fetch --quiet origin main
	@test "$$(git rev-parse HEAD)" = "$$(git rev-parse origin/main)" || { echo "main differs from origin/main: push or pull first"; exit 1; }
	@case "$(V)" in *-rc.*) ;; *) ../../.github/scripts/changelog-section.sh CHANGELOG.md "$(V)" >/dev/null || exit 1 ;; esac
	git tag -a "server/v$(V)" -m "server v$(V)"
	git push origin "server/v$(V)" || { git tag -d "server/v$(V)"; exit 1; }
