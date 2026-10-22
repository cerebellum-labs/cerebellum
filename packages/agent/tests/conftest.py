from __future__ import annotations

import io
import time
from collections.abc import Callable, Iterator

import pytest
from fake_server import FakeServer
from PIL import Image

from cerebellum import Cerebellum

RULES = {
    "tasks": {
        "sort_item": {
            "options": ["basket", "discard"],
            "prompt": "Is the object in the photo a fruit? Answer basket if it is a fruit, discard otherwise.",
        },
    },
    "actions": {
        "basket": {"class": "reversible", "review_below": 0.8},
        "discard": {"class": "irreversible"},
        "stop": {"class": "reflex"},
    },
}


def jpeg(width: int = 640, height: int = 480) -> bytes:
    out = io.BytesIO()
    Image.new("RGB", (width, height), (200, 120, 20)).save(out, format="JPEG", quality=90)
    return out.getvalue()


def wait_for(condition: Callable[[], bool], timeout: float = 5.0) -> None:
    deadline = time.monotonic() + timeout
    while time.monotonic() < deadline:
        if condition():
            return
        time.sleep(0.01)
    raise AssertionError("condition not met in time")


@pytest.fixture
def server() -> Iterator[FakeServer]:
    s = FakeServer()
    yield s
    s.close()


@pytest.fixture
def make_agent(server: FakeServer, tmp_path) -> Iterator[Callable[..., Cerebellum]]:
    agents: list[Cerebellum] = []

    def make(**kwargs) -> Cerebellum:
        options = {
            "server_url": server.url,
            "token": server.token,
            "db_path": tmp_path / "agent.db",
            "sync_interval": 0.05,
            "decide_timeout": 2,
            "sync_timeout": 2,
        }
        agent = Cerebellum(kwargs.pop("rules", RULES), **{**options, **kwargs})
        agents.append(agent)
        return agent

    yield make
    for agent in agents:
        agent.stop()
