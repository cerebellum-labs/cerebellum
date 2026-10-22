"""The real agent and the real sorter against a fake server: the demo's offline scenes, without hardware."""

from __future__ import annotations

import sys
import time
from pathlib import Path

import pytest

sys.path.insert(0, str(Path(__file__).resolve().parents[3] / "packages" / "agent" / "tests"))
from cerebellum import Cerebellum  # noqa: E402
from fake_server import FakeServer  # noqa: E402

from fruit_sorter.cli import RULES  # noqa: E402
from fruit_sorter.config import Config  # noqa: E402
from fruit_sorter.hardware import LightPanel, NullDriver  # noqa: E402
from fruit_sorter.presence import ZonePresence  # noqa: E402
from fruit_sorter.sim import SimDetector, SimWorld, render  # noqa: E402
from fruit_sorter.sorter import Sorter  # noqa: E402


class Servo:
    def __init__(self) -> None:
        self.sweeps: list[str] = []

    def sweep(self, action):
        self.sweeps.append(action)


class Button:
    def __init__(self) -> None:
        self.presses = 0

    def pressed(self):
        if self.presses:
            self.presses -= 1
            return True
        return False


class Robot:
    def __init__(self, server: FakeServer, tmp_path) -> None:
        self.cfg = Config(warmup_frames=3, dwell_s=0.2, leave_s=0.3, hold_s=0.3, person_clear_s=0.2)
        self.world = SimWorld()
        self.servo, self.button = Servo(), Button()
        self.agent = Cerebellum(
            RULES, server_url=server.url, token=server.token, db_path=tmp_path / "a.db",
            sync_interval=0.05, decide_timeout=2, sync_timeout=2,
        )  # fmt: skip
        self.lights = LightPanel(NullDriver(), self.cfg.blink_period_s)
        self.sorter = Sorter(
            self.agent,
            SimDetector(self.world),
            ZonePresence(self.cfg.zone),
            self.lights,
            self.servo,
            self.button,
            self.cfg,
        )
        self.sorter.attach()
        self.agent.start()

    def show(self, scene: str, until=None, seconds: float = 0.5, timeout: float = 8.0) -> None:
        """Show a scene for `seconds`, or until `until()` holds."""
        self.world.scene = scene
        frame = render(scene)
        end = time.monotonic() + (timeout if until else seconds)
        while time.monotonic() < end:
            self.sorter.step(frame)
            if until and until():
                return
            time.sleep(0.03)
        if until:
            raise AssertionError(f"not reached while showing {scene}")

    def lit(self, name: str) -> bool:
        return self.lights.is_on(name, time.monotonic())


@pytest.fixture
def server():
    s = FakeServer()
    yield s
    s.close()


@pytest.fixture
def robot(server, tmp_path):
    r = Robot(server, tmp_path)
    r.show("empty", seconds=0.8)
    yield r
    r.agent.stop()


def test_a_fruit_is_sorted_on_the_device_and_the_server_hears_of_it(robot, server):
    robot.show("orange", until=lambda: robot.servo.sweeps)
    assert robot.servo.sweeps == ["basket"]
    robot.show("orange", until=lambda: any(d.get("executed_at") for d in server.decisions.values()))
    (d,) = server.decisions.values()
    assert (
        d["action"] == "basket" and d["action_class"] == "reversible" and d["metadata"]["detector"]["label"] == "orange"
    )
    assert server.frame_uploads[d["id"]] == 1


def test_an_item_asked_about_offline_waits_and_is_sorted_when_the_network_returns(robot, server):
    server.online = False
    server.decide_action = "basket"  # the server will know the kiwi is a fruit
    robot.show("empty", seconds=0.5)
    robot.show("kiwi", until=lambda: robot.sorter.pending is not None)
    robot.show("kiwi", until=lambda: not robot.agent.is_online)
    assert robot.lights.mode("yellow") == "blink" and robot.servo.sweeps == []

    robot.show("kiwi", seconds=1.0)  # still waiting, and the robot hasn't given up
    assert robot.sorter.pending is not None and robot.servo.sweeps == []

    server.online = True
    robot.show("kiwi", until=lambda: robot.servo.sweeps)
    assert robot.servo.sweeps == ["basket"]
    robot.show("kiwi", until=lambda: any(d.get("executed_at") for d in server.decisions.values()))
    assert robot.lights.mode("yellow") == "off"


def test_the_item_taken_away_while_offline_is_cancelled_not_acted_on(robot, server):
    server.online = False
    robot.show("cup", until=lambda: robot.sorter.pending is not None)
    robot.show("empty", until=lambda: robot.sorter.pending is None)
    server.online = True
    robot.show("empty", until=lambda: any(d.get("cancelled_at") for d in server.decisions.values()))
    assert robot.servo.sweeps == []


def test_a_review_that_disagrees_makes_the_white_light_blink_until_the_button(robot, server):
    server.review_action = "discard"
    robot.show("uncertain_orange", until=lambda: robot.sorter.corrections_pending)
    robot.show("empty", until=lambda: robot.lights.mode("white") == "blink")
    robot.button.presses = 1
    robot.show("empty", until=lambda: robot.lights.mode("white") == "off")
    robot.show("empty", until=lambda: any(d.get("correction_applied_at") for d in server.decisions.values()))


def test_a_hand_is_a_reflex_in_the_journal_once(robot, server):
    robot.show("person", seconds=1.0)
    robot.show("empty", seconds=0.6)
    reflexes = [d for d in server.decisions.values() if d["action_class"] == "reflex"]
    assert len(reflexes) == 1 and reflexes[0]["options"] == ["stop"] and reflexes[0]["executed_at"]
