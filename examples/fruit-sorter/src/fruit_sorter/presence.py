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
        self,
        zone: tuple[float, float, float, float],
        threshold: int = 28,
        min_area: float = 0.012,
        learn: float = 0.05,
        max_light_shift: float = 25,
        edge_margin: int = 4,
        min_side: int = 40,
        max_aspect: float = 4.0,
        max_area: float = 0.8,
    ) -> None:
        self._max_light_shift = max_light_shift
        # What an item looks like, in the zone's pixels. A shadow or an arm comes in from outside, so it touches the
        # edge of the zone and is long and thin; a change of light covers nearly everything. An item is put inside.
        self._edge_margin = edge_margin
        self._min_side = min_side
        self._max_aspect = max_aspect
        self._max_area = max_area
        self._zone = zone
        self._threshold = threshold
        self._min_area = min_area
        self._learn = learn
        self._background: np.ndarray | None = None
        # The picture of the zone from before the last absorb(): what it looks like once what was absorbed is gone.
        self._before_absorb: np.ndarray | None = None
        self._last: np.ndarray | None = None

    def rect(self, shape: tuple[int, ...]) -> tuple[int, int, int, int]:
        h, w = shape[:2]
        x0, y0, x1, y1 = self._zone
        return int(x0 * w), int(y0 * h), int(x1 * w), int(y1 * h)

    def reset(self) -> None:
        """Forget the picture of the empty zone; the next frame becomes it."""
        self._background = None
        self._before_absorb = None

    def absorb(self) -> None:
        """Take what is in the zone now for part of the scene, but remember what the zone looked like before: when
        it is taken away, that is a removal, not a new item."""
        if self._background is not None and self._last is not None:
            self._before_absorb = self._background
            self._background = self._last.copy()

    def update(self, frame: np.ndarray, learn_if_empty: bool = True) -> Item | None:
        x0, y0, x1, y1 = self.rect(frame.shape)
        gray = cv2.GaussianBlur(cv2.cvtColor(frame[y0:y1, x0:x1], cv2.COLOR_BGR2GRAY), (9, 9), 0).astype(np.float32)
        self._last = gray
        if self._background is None:
            self._background = gray.copy()
            return None
        diff = gray - self._background
        # The whole zone getting lighter or darker (a cloud, the camera's exposure) isn't an item: take the typical
        # change off first. Bounded, so a big bright item isn't mistaken for the light.
        diff -= float(np.clip(np.median(diff), -self._max_light_shift, self._max_light_shift))
        changed = (np.abs(diff) > self._threshold).astype(np.uint8)
        opened = cv2.morphologyEx(changed, cv2.MORPH_OPEN, np.ones((5, 5), np.uint8))
        mask = cv2.morphologyEx(opened, cv2.MORPH_CLOSE, np.ones((15, 15), np.uint8))
        contours, _ = cv2.findContours(mask, cv2.RETR_EXTERNAL, cv2.CHAIN_APPROX_SIMPLE)
        zone_area = mask.shape[0] * mask.shape[1]
        biggest = max(
            (c for c in contours if self._is_item_shaped(c, mask.shape, zone_area)), key=cv2.contourArea, default=None
        )
        if biggest is None:
            if learn_if_empty:
                # Follows the light slowly; the shift already taken into account keeps it from lagging behind. Only
                # where nothing changed: an item the zone could not take for one (a hand carrying it in) must not be
                # learned into the scene, or taking it away would leave a ghost.
                unchanged = (np.abs(diff) < self._threshold * 0.5).astype(np.uint8)
                cv2.accumulateWeighted(gray, self._background, self._learn, mask=unchanged)
            return None
        x, y, w, h = cv2.boundingRect(biggest)
        if self._before_absorb is not None and self._looks_like(gray, self._before_absorb, (x, y, w, h)):
            # What was absorbed has been taken away: the zone is as it was before, and the "item" is its hole.
            self._background, self._before_absorb = self._before_absorb, None
            return None
        return Item((x0 + x, y0 + y, x0 + x + w, y0 + y + h), cv2.contourArea(biggest) / zone_area)

    def _looks_like(self, gray: np.ndarray, other: np.ndarray, rect: tuple[int, int, int, int]) -> bool:
        """Does the part of the zone in `rect` look the same as in `other`, give or take the light?"""
        x, y, w, h = rect
        diff = gray[y : y + h, x : x + w] - other[y : y + h, x : x + w]
        diff -= float(np.clip(np.median(gray - other), -self._max_light_shift, self._max_light_shift))
        return float(np.abs(diff).mean()) < self._threshold * 0.5

    def _is_item_shaped(self, contour: np.ndarray, shape: tuple[int, ...], zone_area: int) -> bool:
        area = cv2.contourArea(contour) / zone_area
        if not self._min_area <= area <= self._max_area:
            return False
        x, y, w, h = cv2.boundingRect(contour)
        m = self._edge_margin
        if x < m or y < m or x + w > shape[1] - m or y + h > shape[0] - m:
            return False
        return min(w, h) >= self._min_side and max(w, h) / min(w, h) <= self._max_aspect
