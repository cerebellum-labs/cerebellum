# Third-party notices

Cerebellum's own code is under the [MIT License](LICENSE). It uses the components below, which keep their own licenses and terms. None of them is stored in this repository; the server image is the only place one is redistributed.

| Component | How | License or terms |
| --- | --- | --- |
| NVIDIA Nemotron | Called through the Nebius Token Factory API, not downloaded | [NVIDIA Open Model License](https://www.nvidia.com/en-us/agreements/enterprise-software/nvidia-open-model-license/) |
| Nebius Token Factory, Nebius AI Cloud | Hosted services | [Nebius legal terms](https://docs.nebius.com/legal) |
| Alpine Linux | Base of the server container image (`ghcr.io/cerebellum-labs/cerebellum-server`) | Each package keeps its own license, among them GPL-2.0 (BusyBox) and MIT (musl); sources at [alpinelinux.org](https://alpinelinux.org) |

Add a row for every model, dataset or library with a license other than MIT, Apache 2.0, BSD or MPL 2.0, before merging the code that uses it.
