# Infra

Runs the server behind a [Cloudflare Tunnel](https://developers.cloudflare.com/cloudflare-one/connections/connect-networks/), so the host needs no open ports.

## Run

1. In Cloudflare Zero Trust, create a tunnel and add a published application with service URL `http://server:8080`.
2. Copy `.env.example` to `.env`: the tunnel token, a database password, the dashboard password and session secret, the dashboard's origin if it runs on a host of its own, and the Nebius Token Factory key.
3. Start:
   ```bash
   docker compose up -d --wait
   ```

`SERVER_TAG` in `.env` picks the server image: `main` or a release version.

The dashboard runs on a host of its own (the demo uses Cloudflare Pages): set `DASHBOARD_ORIGIN` to its origin, build it with `VITE_API_URL` set to `https://<the tunnel's hostname>`, and name that origin in the CSP of its `public/_headers`, as [`packages/dashboard`](../packages/dashboard) describes.

## Operate

```bash
docker compose ps                                  # healthy: the server answers /health
docker compose logs -f server                      # JSON logs
docker compose pull && docker compose up -d --wait # update to the newest image of SERVER_TAG
docker compose exec -T postgres pg_dump -U cerebellum cerebellum > backup.sql
```

The database, frames included, lives in the `postgres-data` volume; `docker compose down` keeps it, `docker compose down -v` deletes it. The server applies its migrations on start, so an update needs no other step.
