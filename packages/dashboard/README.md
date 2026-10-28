# Cerebellum dashboard

The web dashboard of Cerebellum: robots and the journal of their decisions, live.

## Run

Node.js 24 and pnpm (`corepack enable` sets up the pinned version). The dashboard runs at http://localhost:5173 and passes `/api`, the WebSocket included, through to the server.

Against a local server, in two terminals:

```bash
cd packages/server && make db run      # PostgreSQL in Docker, then the server
cd packages/dashboard && pnpm install && pnpm dev
```

Sign in with the password `admin`. Against another server, set its origin: `API_PROXY_TARGET=https://api.cerebellumlabs.dev pnpm dev` shows the live demo's data as it was when the page loaded: that server accepts sign-ins and the live WebSocket only from its own dashboard.

On the in-memory mock with a simulated fruit sorter, no server needed:

```bash
VITE_API_MOCK=true pnpm dev
```

The checks CI runs: `pnpm lint`, `pnpm typecheck` and `pnpm build`.

## Deploy

Cloudflare Pages builds it with `pnpm build` and serves `dist`. Set `VITE_API_URL` to the API's origin, like `https://api.cerebellumlabs.dev`, and on the server set `DASHBOARD_ORIGIN` to the dashboard's. `public/_headers` sets the security headers; its CSP names the API's origin.
