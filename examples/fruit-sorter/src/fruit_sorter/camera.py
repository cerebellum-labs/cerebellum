"""The camera, read on a thread of its own so the robot always gets the latest frame, not one from a queue."""

from __future__ import annotations

import logging
import sys
import threading
import time
from collections.abc import Callable
from typing import Any

import numpy as np

log = logging.getLogger("fruit_sorter")


def open_capture(source: int | str, width: int, height: int) -> Any:
    import cv2

    # On Linux V4L2 is named outright: the default backend search is slow and sometimes picks a worse one.
    capture = cv2.VideoCapture(source, cv2.CAP_V4L2) if sys.platform.startswith("linux") else cv2.VideoCapture(source)
    capture.set(cv2.CAP_PROP_FRAME_WIDTH, width)
    capture.set(cv2.CAP_PROP_FRAME_HEIGHT, height)
    capture.set(cv2.CAP_PROP_BUFFERSIZE, 1)
    return capture


def open_any(width: int, height: int, indexes: range = range(10), opener: Callable[[int], Any] | None = None) -> Any:
    """The first camera that delivers a frame: the number a camera gets changes when it is plugged into another
    port or another camera is added, so the robot looks instead of insisting on one."""
    for index in indexes:
        capture = opener(index) if opener else open_capture(index, width, height)
        if capture.isOpened():
            ok, frame = capture.read()
            if ok and frame is not None:
                log.info("using camera %d", index)
                return capture
        capture.release()
    return None


class Camera:
    """Keeps reading frames; `read()` returns the newest. Reopens the camera if it stops delivering."""

    def __init__(
        self,
        source: int | str = 0,
        width: int = 640,
        height: int = 480,
        opener: Callable[[], Any] | None = None,
        retry_s: float = 1.0,
    ) -> None:
        if opener is not None:
            self._opener = opener
        elif source == "auto":
            self._opener = lambda: open_any(width, height) or _Closed()
        else:
            self._opener = lambda: open_capture(source, width, height)
        self._retry_s = retry_s
        self._lock = threading.Lock()
        self._frame: np.ndarray | None = None
        self._stop = threading.Event()
        self._thread = threading.Thread(target=self._run, name="camera", daemon=True)

    def start(self) -> Camera:
        self._thread.start()
        return self

    def read(self, timeout: float = 0.0) -> np.ndarray | None:
        """The newest frame, or None if there's none yet (waiting up to `timeout` seconds for the first)."""
        deadline = time.monotonic() + timeout
        while True:
            with self._lock:
                frame = self._frame
            if frame is not None or time.monotonic() >= deadline:
                return frame
            time.sleep(0.02)

    def close(self) -> None:
        self._stop.set()
        self._thread.join(timeout=3)

    def _run(self) -> None:
        capture = None
        failures = 0
        while not self._stop.is_set():
            if capture is None:
                capture = self._opener()
                if not capture.isOpened():
                    log.error("can't open the camera, trying again")
                    capture.release()
                    capture = None
                    self._stop.wait(self._retry_s)
                    continue
            try:
                ok, frame = capture.read()
            except Exception:
                log.exception("reading the camera failed")
                ok, frame = False, None
            if ok and frame is not None:
                failures = 0
                with self._lock:
                    self._frame = frame
                continue
            failures += 1
            if failures >= 5:
                log.warning("the camera stopped delivering frames, reopening it")
                capture.release()
                capture, failures = None, 0
                with self._lock:
                    self._frame = None  # a frozen picture must not look alive
            self._stop.wait(0.1)
        if capture is not None:
            capture.release()


class _Closed:
    """What an opener returns when no camera was found, so the retry logic treats it like any closed camera."""

    def isOpened(self) -> bool:  # noqa: N802 - OpenCV's name
        return False

    def release(self) -> None:
        pass
