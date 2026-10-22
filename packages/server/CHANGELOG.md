# Changelog

All notable changes to the Cerebellum server are listed here, in the [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) format. Versions follow [Semantic Versioning](https://semver.org/spec/v2.0.0.html) and are released as `server/vX.Y.Z` tags; 0.x until the agent–server API settles.

## [Unreleased]

### Added

- Health check at `/health`, JSON errors, `X-Request-ID` on every response, 5 MB request body limit.
- Container image for `linux/amd64` and `linux/arm64` on GitHub Container Registry: `ghcr.io/cerebellum-labs/cerebellum-server`.
- PostgreSQL 18 database, required: `DATABASE_URL`. The server applies its migrations on start.

[Unreleased]: https://github.com/cerebellum-labs/cerebellum/commits/main/packages/server
