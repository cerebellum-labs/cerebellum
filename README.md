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

## Getting started

Coming with the code.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md). Releases: [RELEASING.md](RELEASING.md). Security issues: [SECURITY.md](SECURITY.md). Third-party models and their licenses: [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md).

## License

[MIT](LICENSE)
