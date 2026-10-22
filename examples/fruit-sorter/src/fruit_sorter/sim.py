"""A made-up world for trying the sorter without a camera: scenes drawn as flat shapes and a detector that
knows what each one is. The demo's items, as the spec describes them."""

from __future__ import annotations

from dataclasses import dataclass

import cv2
import numpy as np

from .detector import Detection

WIDTH, HEIGHT = 640, 480
CENTER = (320, 264)
BOX = (270, 214, 370, 314)
SCENES = ("empty", "orange", "uncertain_orange", "cup", "kiwi", "person")


@dataclass
class SimWorld:
    scene: str = "empty"


def render(scene: str) -> np.ndarray:
    frame = np.full((HEIGHT, WIDTH, 3), 205, np.uint8)
    if scene in ("orange", "uncertain_orange"):
        cv2.circle(frame, CENTER, 50, (30, 140, 250), -1)
    elif scene == "cup":
        cv2.rectangle(frame, (275, 214), (365, 314), (250, 250, 250), -1)
    elif scene == "kiwi":
        cv2.ellipse(frame, CENTER, (55, 40), 0, 0, 360, (50, 100, 80), -1)
    elif scene == "person":
        cv2.rectangle(frame, (0, 60), (230, HEIGHT), (120, 130, 190), -1)
    elif scene != "empty":
        raise ValueError(f"unknown scene {scene!r}")
    return frame


class SimDetector:
    name = "sim-detector"

    def __init__(self, world: SimWorld) -> None:
        self._world = world

    def detect(self, frame: np.ndarray) -> list[Detection]:
        return {
            "orange": [Detection("orange", 0.93, BOX)],
            "uncertain_orange": [Detection("orange", 0.62, BOX)],
            "cup": [Detection("cup", 0.88, BOX)],
            "kiwi": [],  # not in COCO: the detector has nothing to say
            "person": [Detection("person", 0.9, (0, 60, 230, HEIGHT))],
        }.get(self._world.scene, [])
