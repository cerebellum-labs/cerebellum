# Third-party notices

Cerebellum's own code is under the [MIT License](LICENSE). It uses the components below, which keep their own licenses and terms. None of them is stored in this repository; the server image is the only place one is redistributed.

| Component | How | License or terms |
| --- | --- | --- |
| NVIDIA Nemotron | Called through the Nebius Token Factory API, not downloaded | [NVIDIA Open Model License](https://www.nvidia.com/en-us/agreements/enterprise-software/nvidia-open-model-license/) |
| Nebius Token Factory | Hosted service | [Nebius legal terms](https://docs.nebius.com/legal) |
| YOLOX-Nano | Object detector of the fruit sorter example, run on the device through ONNX Runtime; weights are downloaded by `fruit-sorter download-model`, not stored here | [Apache License 2.0](https://github.com/Megvii-BaseDetection/YOLOX/blob/main/LICENSE) |
| COCO | The dataset YOLOX was trained on; only its 80 class names are used | Annotations under [CC BY 4.0](https://cocodataset.org/#termsofuse) |
| Pillow | Python library of the agent: shrinks camera frames | [MIT-CMU](https://github.com/python-pillow/Pillow/blob/main/LICENSE) (permissive, like MIT) |
| OpenCV (`opencv-python-headless`) | Python library of the fruit sorter example: camera and image work | [Apache License 2.0](https://github.com/opencv/opencv/blob/4.x/LICENSE); the wheels bundle FFmpeg libraries under the [LGPL 2.1](https://github.com/opencv/opencv-python/blob/4.x/LICENSE-3RD-PARTY.txt) |
| Alpine Linux | Base of the server container image (`ghcr.io/cerebellum-labs/cerebellum-server`) | Each package keeps its own license, among them GPL-2.0 (BusyBox) and MIT (musl); sources at [alpinelinux.org](https://alpinelinux.org) |

Add a row for every model, dataset or library with a license other than MIT, Apache 2.0, BSD or MPL 2.0, before merging the code that uses it.
