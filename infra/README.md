# Infra

Runs the server behind a [Cloudflare Tunnel](https://developers.cloudflare.com/cloudflare-one/connections/connect-networks/), so the host needs no open ports.

## Run

1. In Cloudflare Zero Trust, create a tunnel and add a published application with service URL `http://server:8080`.
2. Copy `.env.example` to `.env`: the tunnel token, a database password, the dashboard password and session secret, and the dashboard's origin if it runs on a host of its own.
3. Start:
   ```bash
   docker compose up -d --wait
   ```

`SERVER_TAG` in `.env` picks the server image: `main` or a release version.
