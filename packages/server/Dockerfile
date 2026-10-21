# syntax=docker/dockerfile:1

FROM --platform=$BUILDPLATFORM golang:1.27-alpine AS builder
WORKDIR /src

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY . .

ARG TARGETOS
ARG TARGETARCH
ARG VERSION=dev

# Both caches are mounted here too: without the module cache the build would
# re-download everything go mod download just fetched.
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -trimpath \
    -ldflags="-s -w -X main.version=${VERSION}" \
    -o /out/server ./cmd/server


# alpine already ships the root certificates outbound TLS needs
# (ca-certificates-bundle). With no RUN in this stage, the arm64 image is
# assembled without emulating arm64.
FROM alpine:3

COPY --from=builder /out/server /server

# A numeric user needs no /etc/passwd entry; it just must not be root.
USER 10001:10001
ENTRYPOINT ["/server"]
