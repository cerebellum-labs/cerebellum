BINARY := bin/server
# Release tags carry the package prefix (server/v1.2.3), so only those are
# matched and the prefix is cut: 1.2.3 on a release tag, 1.2.3-4-gabc1234
# after it, the commit hash before the first release.
VERSION ?= $(or $(patsubst server/v%,%,$(shell git describe --tags --match 'server/v*' --always --dirty 2>/dev/null)),dev)
LDFLAGS := -X main.version=$(VERSION)

# sqlc compiles internal/db/queries into Go; run at a pinned version, the
# same here and in CI, so the generated code doesn't change between machines.
SQLC := go run github.com/sqlc-dev/sqlc/cmd/sqlc@v1.31.1

# The database from `make db`.
LOCAL_DATABASE_URL := postgres://postgres@localhost:5432/postgres?sslmode=disable

.PHONY: run build test test-integration lint sqlc-diff generate db fmt tidy clean docker release

# Runs against the database from `make db` unless DATABASE_URL is set.
run: export DATABASE_URL ?= $(LOCAL_DATABASE_URL)
run:
	go run ./cmd/server

build:
	go build -ldflags "$(LDFLAGS)" -o $(BINARY) ./cmd/server

test:
	go test ./... -race -shuffle=on -cover

# The tests tagged integration as well, against PostgreSQL: the database from
# `make db` unless TEST_DATABASE_URL is set. -count=1: never cached, since the
# result depends on the database, not only on the code.
test-integration: export TEST_DATABASE_URL ?= $(LOCAL_DATABASE_URL)
test-integration:
	go test ./... -tags=integration -race -shuffle=on -count=1 -cover

# golangci-lint runs go vet among its linters; .golangci.yml is the config.
lint: sqlc-diff
	golangci-lint run ./...

# Fails when the generated code is behind the queries or the migrations.
sqlc-diff:
	$(SQLC) diff

generate:
	$(SQLC) generate

# PostgreSQL 18 in Docker, without a password, for local development only.
# Returns once it accepts connections, so `make db test-integration` works.
db:
	docker run --rm -d --name cerebellum-postgres -p 127.0.0.1:5432:5432 \
		-e POSTGRES_HOST_AUTH_METHOD=trust postgres:18-alpine
	until docker exec cerebellum-postgres pg_isready -q -U postgres; do sleep 1; done

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
