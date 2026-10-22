<div align="center">

# Cerebellum

**Sync layer between a robot's on-device and cloud models.**

[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

</div>

Every robotics team writes its own glue between the model on the device and the model in the cloud, and that glue breaks when the network drops. Cerebellum is that glue, ready-made: it routes each decision by your rules, queues requests while offline, and syncs when the network is back.

> Built for the [Nebius x NVIDIA Global AI Hackathon](https://nebiusglobalaihackathon.devpost.com/), Physical AI track. Work in progress.

## Repository layout

| Path | What |
| --- | --- |
| [`packages/agent`](packages/agent) | On-device library |
| [`packages/server`](packages/server) | Cloud server |
| [`packages/dashboard`](packages/dashboard) | Web dashboard |
| [`examples/fruit-sorter`](examples/fruit-sorter) | Demo robot on a Raspberry Pi |
| [`infra`](infra) | Docker compose for the server behind a Cloudflare Tunnel |

## How it works

```
 robot code ──▶ agent (on the device) ──HTTPS──▶ server (cloud) ──▶ Nemotron (Nebius Token Factory)
 camera, motors   rules, SQLite journal and queue   journal, review          decides and reviews
 local model      sync in the background            dashboard (live)
```

The robot hands the agent a task, a frame and its local model's answer. The agent routes it by your rules:

| Class | Example | Who decides | Without a network |
| --- | --- | --- | --- |
| Reflex | stop when a person is near | the robot, at once; the agent only journals it | works |
| Reversible | put an item in the basket | the agent, from the local model; the server reviews the uncertain ones later and sends a correction if it disagrees | works |
| Irreversible | throw an item away | only the server, which asks the cloud model | waits in the queue |

The server never calls the device, so a robot behind NAT or a phone hotspot works: whatever the server has for it comes back in a response. Every decision is journalled on the device first and synced later, and the dashboard shows all devices live: where each decision was made, by which model, how sure, how fast and at what cost.

**Models.** The cloud model is NVIDIA Nemotron 3.5 Super VL, called through the OpenAI-compatible API of [Nebius Token Factory](https://tokenfactory.nebius.com); it decides irreversible actions and reviews uncertain ones, looking at the robot's frame. The demo robot's local model is YOLOX-Nano, which runs on the Raspberry Pi.

## Getting started

1. **Server.** Deploy it with [`infra`](infra) (Docker, PostgreSQL, a Cloudflare Tunnel), or locally with `make db run` in [`packages/server`](packages/server). Open the dashboard, sign in and add a device: it shows the device's token once.
2. **Agent.** Python 3.11+ and [uv](https://docs.astral.sh/uv/). Put `CEREBELLUM_URL` and `CEREBELLUM_TOKEN` in the robot's environment and see [`packages/agent`](packages/agent) for the API; the wire format is [`AGENT_API.md`](packages/server/AGENT_API.md).
3. **Try it.** The [fruit sorter](examples/fruit-sorter) plays the demo's items on a made-up camera, no hardware needed:

   ```bash
   cd examples/fruit-sorter
   uv sync && cp .env.example .env     # then put the device token in .env
   uv run fruit-sorter sim
   ```

   On a Raspberry Pi with a camera, LEDs and a servo, the same code drives the real robot.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md). Releases: [RELEASING.md](RELEASING.md). Security issues: [SECURITY.md](SECURITY.md). Third-party models and their licenses: [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md).

## License

[MIT](LICENSE)
