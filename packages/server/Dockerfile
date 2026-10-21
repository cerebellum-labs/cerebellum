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


FROM alpine:3
# ca-certificates is required: outbound calls go over TLS, and the base image
# ships no root certificates.
RUN apk add --no-cache ca-certificates \
    && adduser -D -H -u 10001 app

COPY --from=builder /out/server /server

USER app
ENTRYPOINT ["/server"]
