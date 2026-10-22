"""Is there something in the zone? Answered without the detector, so a kiwi COCO has never heard of still counts."""

from __future__ import annotations

from dataclasses import dataclass

import cv2
import numpy as np


@dataclass(frozen=True)
class Item:
    box: tuple[int, int, int, int]  # x0, y0, x1, y1 in the frame
    area_fraction: float  # of the zone


class ZonePresence:
    """Compares the zone with a picture of it empty, kept up to date while nothing is there."""

    def __init__(
        self, zone: tuple[float, float, float, float], threshold: int = 28, min_area: float = 0.012, learn: float = 0.05
    ) -> None:
        self._zone = zone
        self._threshold = threshold
        self._min_area = min_area
        self._learn = learn
        self._background: np.ndarray | None = None

    def rect(self, shape: tuple[int, ...]) -> tuple[int, int, int, int]:
        h, w = shape[:2]
        x0, y0, x1, y1 = self._zone
        return int(x0 * w), int(y0 * h), int(x1 * w), int(y1 * h)

    def reset(self) -> None:
        self._background = None

    def update(self, frame: np.ndarray, learn_if_empty: bool = True) -> Item | None:
        x0, y0, x1, y1 = self.rect(frame.shape)
        gray = cv2.GaussianBlur(cv2.cvtColor(frame[y0:y1, x0:x1], cv2.COLOR_BGR2GRAY), (9, 9), 0).astype(np.float32)
        if self._background is None:
            self._background = gray.copy()
            return None
        changed = (cv2.absdiff(gray, self._background) > self._threshold).astype(np.uint8)
        opened = cv2.morphologyEx(changed, cv2.MORPH_OPEN, np.ones((5, 5), np.uint8))
        mask = cv2.morphologyEx(opened, cv2.MORPH_CLOSE, np.ones((15, 15), np.uint8))
        contours, _ = cv2.findContours(mask, cv2.RETR_EXTERNAL, cv2.CHAIN_APPROX_SIMPLE)
        zone_area = mask.shape[0] * mask.shape[1]
        biggest = max(contours, key=cv2.contourArea, default=None)
        if biggest is None or cv2.contourArea(biggest) / zone_area < self._min_area:
            if learn_if_empty:
                cv2.accumulateWeighted(gray, self._background, self._learn)
            return None
        x, y, w, h = cv2.boundingRect(biggest)
        return Item((x0 + x, y0 + y, x0 + x + w, y0 + y + h), cv2.contourArea(biggest) / zone_area)
