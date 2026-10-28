# Changelog

All notable changes to the Cerebellum server are listed here, in the [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) format. Versions follow [Semantic Versioning](https://semver.org/spec/v2.0.0.html) and are released as `server/vX.Y.Z` tags; 0.x until the agent–server API settles.

## [Unreleased]

## [0.1.0] - 2026-10-28

### Added

- Health check at `/health`, JSON errors, `X-Request-ID` on every response, 5 MB request body limit.
- Container image for `linux/amd64` and `linux/arm64` on GitHub Container Registry: `ghcr.io/cerebellum-labs/cerebellum-server`.
- PostgreSQL 18 database, required: `DATABASE_URL`. The server applies its migrations on start.
- Agent API under `/api/v1`, see [AGENT_API.md](AGENT_API.md): `/decide` asks the cloud model for irreversible decisions, `/sync` takes everything else and returns corrections. Devices sign in with their token. Uncertain decisions are reviewed in the background, and a review that disagrees becomes a correction. `/decide` asks without reasoning for a quick answer; the review lets the model reason. Replies are requested in JSON mode. The cloud model is set by `NEBIUS_API_KEY`, with `NEBIUS_BASE_URL`, `MODEL_ID` and the `MODEL_PRICE_*_PER_M` prices defaulting to Nemotron 3.5 Super VL on Nebius Token Factory.
- Dashboard API under `/api/dashboard`: devices, the journal of decisions with frames, statistics and a WebSocket stream of changes. Changing devices takes `ADMIN_PASSWORD`; the session is a cookie signed with `SESSION_SECRET`. `DASHBOARD_ORIGIN` lets a dashboard on a host of its own call the API.
- `local_latency_ms` in the decision the agent sends: how long the local model took. It comes back as `latency_ms` of a device decision, and the dashboard's statistics average it.
- The journal filters by `source`, and `changed=true` keeps the decisions where the cloud disagreed with the robot.
- `cloud_cost_usd` in every decision: all it cost in the cloud, retries, failed calls and the review included.
- A decision's detail in the dashboard API lists every call made to the cloud model for it, with the raw reply or the error.

### Changed

- AGENT_API.md: a timeout, a 5xx or no network is no longer a failed attempt of an irreversible decision; it waits for the network, and only a `retry: true` answer counts. Documentation only: the server keeps no attempts.

[Unreleased]: https://github.com/cerebellum-labs/cerebellum/compare/server/v0.1.0...HEAD
[0.1.0]: https://github.com/cerebellum-labs/cerebellum/releases/tag/server/v0.1.0
