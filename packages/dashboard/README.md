# Cerebellum dashboard

The web dashboard of Cerebellum: devices and the journal of their decisions, live.

## Run

Against the server, which Vite proxies at `/api`:

```bash
cd ../server && make db run   # PostgreSQL in Docker, then the server; password: admin
pnpm install && pnpm dev
```

On the in-memory mock with a simulated fruit sorter, no server needed:

```bash
VITE_API_MOCK=true pnpm dev
```

## Deploy

Cloudflare Pages builds it with `pnpm build` and serves `dist`. Set `VITE_API_URL` to the API's origin, like `https://api.cerebellumlabs.dev`, and on the server set `DASHBOARD_ORIGIN` to the dashboard's. `public/_headers` sets the security headers; its CSP names the API's origin.
