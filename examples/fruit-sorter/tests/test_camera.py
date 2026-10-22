from __future__ import annotations

import time

import numpy as np

from fruit_sorter.camera import Camera


class FakeCapture:
    def __init__(self, frames: int, opened: bool = True) -> None:
        self.frames = frames
        self.opened = opened
        self.released = False

    def isOpened(self) -> bool:  # noqa: N802 - OpenCV's name
        return self.opened

    def read(self):
        time.sleep(0.005)
        if self.frames <= 0:
            return False, None
        self.frames -= 1
        return True, np.full((4, 4, 3), self.frames % 256, np.uint8)

    def release(self) -> None:
        self.released = True


def wait_for(condition, timeout=3.0):
    deadline = time.monotonic() + timeout
    while time.monotonic() < deadline:
        if condition():
            return
        time.sleep(0.01)
    raise AssertionError("condition not met in time")


def test_read_returns_the_newest_frame():
    camera = Camera(opener=lambda: FakeCapture(200)).start()
    try:
        first = camera.read(timeout=2)
        assert first is not None
        wait_for(lambda: int(camera.read()[0, 0, 0]) < int(first[0, 0, 0]))
    finally:
        camera.close()


def test_a_camera_that_stops_is_reopened_and_never_shows_a_frozen_frame():
    captures = [FakeCapture(3), FakeCapture(200)]
    opened = []

    def opener():
        opened.append(captures[min(len(opened), 1)])
        return opened[-1]

    camera = Camera(opener=opener, retry_s=0.01).start()
    try:
        wait_for(lambda: len(opened) == 2)
        assert opened[0].released
        wait_for(lambda: camera.read() is not None)
    finally:
        camera.close()


def test_a_camera_that_wont_open_is_retried():
    attempts = []

    def opener():
        attempts.append(1)
        return FakeCapture(100, opened=len(attempts) >= 3)

    camera = Camera(opener=opener, retry_s=0.01).start()
    try:
        assert camera.read(timeout=3) is not None
        assert len(attempts) == 3
    finally:
        camera.close()


def test_auto_picks_the_first_camera_that_delivers_and_closes_the_others():
    from fruit_sorter.camera import open_any

    captures = {0: FakeCapture(100, opened=False), 1: FakeCapture(0), 2: FakeCapture(100), 3: FakeCapture(100)}
    chosen = open_any(640, 480, range(4), opener=lambda i: captures[i])
    assert chosen is captures[2]
    assert captures[0].released and captures[1].released and not captures[3].released


def test_auto_finds_nothing_when_no_camera_works():
    from fruit_sorter.camera import open_any

    assert open_any(640, 480, range(3), opener=lambda i: FakeCapture(0, opened=False)) is None
