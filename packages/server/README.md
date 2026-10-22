# Cerebellum server

The cloud server of Cerebellum: one process with the HTTP API for devices and the dashboard, and the background work behind them.

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

Running it locally and the checks: [CONTRIBUTING.md](../../CONTRIBUTING.md).
