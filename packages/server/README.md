# Cerebellum server

The cloud server of Cerebellum: one process with the HTTP API for robots and the dashboard, and the background work behind them.

## Run

Go 1.27 and Docker. PostgreSQL 18 in Docker, then the server on port 8080, with development-only defaults: the dashboard password `admin`, a fixed session secret, and the local dashboard's origin allowed:

```bash
NEBIUS_API_KEY=<your key> make db run
NEBIUS_API_KEY=<your key> make run      # next time, while the database is still up
```

The database container keeps running until `docker stop cerebellum-postgres`, which also deletes its data.

In production the server runs from its image, `ghcr.io/cerebellum-labs/cerebellum-server` (`linux/amd64` and `linux/arm64`, as a non-root user); [`infra`](../../infra) runs it with PostgreSQL behind a Cloudflare Tunnel. It applies its migrations on start.

## Configuration

Environment variables:

| Variable | Default | What |
| --- | --- | --- |
| `DATABASE_URL` | required | PostgreSQL connection string |
| `ADMIN_PASSWORD` | required | Unlocks adding and deleting robots in the dashboard |
| `SESSION_SECRET` | required | Signs the dashboard's admin cookie; at least 32 bytes |
| `DASHBOARD_ORIGIN` | empty | The dashboard's origin, like `https://demo.cerebellumlabs.dev`, when it runs on a host of its own; empty when it shares the API's origin |
| `NEBIUS_API_KEY` | empty | Token Factory key. Without it, decisions that need the cloud model fail and nothing is reviewed |
| `NEBIUS_BASE_URL` | `https://api.tokenfactory.nebius.com/v1/` | Any OpenAI-compatible API |
| `MODEL_ID` | `nvidia/Nemotron-3.5-Super-VL` | A vision model on that API |
| `MODEL_PRICE_INPUT_PER_M`, `MODEL_PRICE_OUTPUT_PER_M` | `0.30`, `0.90` | USD per million tokens, for the cost the dashboard shows |
| `PORT` | `8080` | HTTP port |

The API for robots is in [AGENT_API.md](AGENT_API.md).

## Layout

| Package | What |
| --- | --- |
| `cmd/server` | Entry point: logger, config, signals, then `app.Run` |
| `internal/app` | Wires everything and runs it: database, migrations, HTTP server and background workers, stopped together |
| `internal/config` | Settings from the environment |
| `internal/db` | Connection pool, migrations (`migrations/`), queries (`queries/`, compiled by sqlc) |
| `internal/server` | HTTP server, router and middleware |
| `internal/agentapi` | The agents' API: `/api/v1/decide` and `/sync`, see [AGENT_API.md](AGENT_API.md) |
| `internal/review` | Background review of uncertain decisions |
| `internal/model` | The cloud model, through an OpenAI-compatible API |
| `internal/dashboard` | The dashboard's API and its event stream |
| `internal/events` | Fans events out to the connected dashboards |
| `internal/view` | The JSON of devices, decisions and corrections |
| `internal/logging` | JSON logger; adds the request id to every record logged with a request's context |
| `internal/httperr` | The JSON error shape of every endpoint |
| `internal/health` | `/health` |

A feature gets a package of its own under `internal/` with its handlers and logic, takes what it needs (`*db.Queries`, `*slog.Logger`) through its constructor, and is registered in `internal/server` or started as a worker in `internal/app`.

The checks (`make lint test`, `make db test-integration`) and the workflow: [CONTRIBUTING.md](../../CONTRIBUTING.md).
