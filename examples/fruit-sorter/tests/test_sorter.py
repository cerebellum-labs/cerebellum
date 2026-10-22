from __future__ import annotations

from typing import Any

import pytest
from cerebellum import Correction, Decision, RulesError

from fruit_sorter.config import Config
from fruit_sorter.hardware import LightPanel, NullDriver
from fruit_sorter.presence import ZonePresence
from fruit_sorter.sim import SimDetector, SimWorld, render
from fruit_sorter.sorter import Sorter


def decision(**fields: Any) -> Decision:
    base = dict(
        id="0199f000-0000-7000-8000-000000000001", task="sort_item", options=["basket", "discard"],
        action_class="reversible", decided_at="2026-10-22T10:00:00.000Z", action="basket",
    )  # fmt: skip
    return Decision(**{**base, **fields})


class StubAgent:
    """Records what the sorter asks of the agent, and answers like the real one does."""

    def __init__(self) -> None:
        self.online = True
        self.calls: list[tuple[str, tuple, dict]] = []
        self.callbacks: dict[str, Any] = {}
        self.cancel_fails = False

    @property
    def is_online(self) -> bool:
        return self.online

    def on_decision(self, fn):
        self.callbacks["decision"] = fn  # noqa: E704

    def on_correction(self, fn):
        self.callbacks["correction"] = fn  # noqa: E704

    def on_connection(self, fn):
        self.callbacks["connection"] = fn  # noqa: E704

    def decide(self, task, **kwargs):
        self.calls.append(("decide", (task,), kwargs))
        proposed, confidence = kwargs["proposed_action"], kwargs.get("local_confidence")
        if proposed == "basket":
            return decision(action="basket", proposed_action="basket", local_confidence=confidence)
        return decision(action=None, action_class="irreversible", proposed_action=proposed)

    def report_reflex(self, task, action, **kwargs):
        self.calls.append(("report_reflex", (task, action), kwargs))
        return decision(action=action, action_class="reflex")

    def mark_executed(self, decision_id):
        self.calls.append(("mark_executed", (decision_id,), {}))

    def cancel(self, decision_id, reason=""):
        self.calls.append(("cancel", (decision_id, reason), {}))
        if self.cancel_fails:
            raise RulesError("already answered")

    def mark_correction_applied(self, correction_id):
        self.calls.append(("mark_correction_applied", (correction_id,), {}))

    def names(self) -> list[str]:
        return [name for name, _, _ in self.calls]

    def kwargs(self, name: str) -> dict:
        return next(kw for n, _, kw in self.calls if n == name)


class Button:
    def __init__(self) -> None:
        self.presses = 0

    def pressed(self) -> bool:
        if self.presses:
            self.presses -= 1
            return True
        return False


class Servo:
    def __init__(self) -> None:
        self.sweeps: list[str] = []

    def sweep(self, action):
        self.sweeps.append(action)


class Rig:
    """A sorter on a made-up world with a fake clock."""

    def __init__(self) -> None:
        self.cfg = Config(warmup_frames=2)
        self.world = SimWorld()
        self.agent = StubAgent()
        self.servo = Servo()
        self.button = Button()
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
        self.now = 100.0
        self.show("empty", 0.3)  # the camera settling, then the empty platform

    def show(self, scene: str, seconds: float) -> None:
        self.world.scene = scene
        frame = render(scene)
        end = self.now + seconds
        while self.now < end:
            self.sorter.step(frame, self.now)
            self.now += 0.1

    def lit(self, name: str) -> bool:
        return self.lights.is_on(name, self.now)

    def answer(self, d: Decision) -> None:
        self.agent.callbacks["decision"](d)


@pytest.fixture
def rig() -> Rig:
    return Rig()


def test_a_sure_fruit_goes_to_the_basket_at_once(rig: Rig):
    rig.show("empty", 1.2)
    rig.show("orange", 1.0)
    assert rig.agent.names() == ["decide", "mark_executed"]
    kw = rig.agent.kwargs("decide")
    assert kw["proposed_action"] == "basket" and kw["local_confidence"] == 0.93
    assert kw["local_model"] == "sim-detector" and kw["metadata"]["detector"]["label"] == "orange"
    assert isinstance(kw["frame"], bytes) and kw["frame"][:2] == b"\xff\xd8"  # a JPEG of the item
    assert rig.servo.sweeps == ["basket"] and rig.lit("green")
    rig.show("orange", 3.0)
    assert rig.agent.names().count("decide") == 1  # one task per item, however long it stays
    assert not rig.lit("green")  # the light goes out after a while


def test_the_task_waits_for_the_item_to_sit_still(rig: Rig):
    rig.show("empty", 1.2)
    rig.show("orange", 0.3)
    assert "decide" not in rig.agent.names()
    rig.show("empty", 1.5)  # gone before half a second
    assert "decide" not in rig.agent.names()


def test_a_cup_waits_for_the_server_then_goes_to_the_bin(rig: Rig):
    rig.show("empty", 1.2)
    rig.show("cup", 1.0)
    kw = rig.agent.kwargs("decide")
    assert kw["proposed_action"] == "discard" and kw["metadata"]["detector"]["label"] == "cup"
    assert rig.lit("yellow") and not rig.lit("blue")  # steady: the server has it
    assert rig.servo.sweeps == []

    rig.answer(decision(action="discard", action_class="irreversible"))
    rig.show("cup", 0.3)
    assert rig.servo.sweeps == ["discard"] and rig.lit("blue") and not rig.lit("yellow")
    assert rig.agent.names()[-1] == "mark_executed"


def test_a_kiwi_the_detector_doesnt_know_still_goes_to_the_server(rig: Rig):
    rig.show("empty", 1.2)
    rig.show("kiwi", 1.0)
    kw = rig.agent.kwargs("decide")
    assert kw["proposed_action"] == "discard" and kw["local_confidence"] is None
    assert kw["metadata"]["detector"]["label"] == "unknown"
    rig.answer(decision(action="basket", action_class="irreversible"))  # the server knew it was a fruit
    rig.show("kiwi", 0.3)
    assert rig.servo.sweeps == ["basket"] and rig.lit("green")


def test_yellow_blinks_when_the_network_is_gone(rig: Rig):
    rig.agent.online = False
    rig.show("empty", 1.2)
    rig.show("cup", 1.0)
    assert rig.lights.mode("yellow") == "blink"
    rig.agent.online = True
    rig.show("cup", 0.2)
    assert rig.lights.mode("yellow") == "on"


def test_item_removed_while_waiting_cancels_the_decision(rig: Rig):
    rig.show("empty", 1.2)
    rig.show("cup", 1.0)
    rig.show("empty", 0.3)
    assert "cancel" in rig.agent.names()
    assert rig.lights.mode("yellow") == "off"
    rig.show("empty", 1.5)
    rig.show("orange", 1.0)
    assert rig.agent.names().count("decide") == 2  # and the next item is a new task


def test_cancelled_by_the_agent_does_nothing(rig: Rig):
    rig.show("empty", 1.2)
    rig.show("cup", 1.0)
    rig.answer(decision(action=None, action_class="irreversible", cancelled_at="2026-10-22T10:00:01.000Z"))
    rig.show("cup", 0.5)
    assert rig.servo.sweeps == [] and rig.lights.mode("yellow") == "off"


def test_a_person_stops_everything_and_is_journalled_once(rig: Rig):
    rig.show("empty", 1.2)
    rig.show("person", 2.0)
    assert rig.agent.names() == ["report_reflex"]  # not once per frame
    task, action = next(args for n, args, _ in rig.agent.calls if n == "report_reflex")
    assert (task, action) == ("safety", "stop") and rig.lit("red") and rig.sorter.person_present
    rig.show("empty", 1.0)
    assert not rig.lit("red") and not rig.sorter.person_present
    rig.show("person", 0.5)
    assert rig.agent.names().count("report_reflex") == 2  # a new appearance is a new entry


def test_nothing_moves_while_a_person_is_in_the_frame(rig: Rig):
    rig.show("empty", 1.2)
    rig.show("cup", 1.0)
    rig.show("person", 0.5)
    rig.answer(decision(action="discard", action_class="irreversible"))
    rig.show("person", 0.5)
    assert rig.servo.sweeps == []
    rig.show("empty", 1.5)
    assert rig.servo.sweeps == ["discard"]  # carried out once the hand is gone


def test_a_correction_blinks_white_until_the_button(rig: Rig):
    c = Correction(id="c1", decision_id="d1", action="discard", reason="a ball", model="m", created_at="t")
    rig.agent.callbacks["correction"](c)
    rig.show("empty", 0.5)
    assert rig.lights.mode("white") == "blink"
    rig.button.presses = 1
    rig.show("empty", 0.2)
    assert ("mark_correction_applied", ("c1",), {}) in rig.agent.calls
    assert rig.lights.mode("white") == "off"


def test_the_crop_is_around_the_item_and_small_enough():
    import cv2
    import numpy as np

    rig = Rig()
    rig.show("empty", 1.2)
    rig.show("orange", 1.0)
    jpeg = rig.agent.kwargs("decide")["frame"]
    image = cv2.imdecode(np.frombuffer(jpeg, np.uint8), cv2.IMREAD_COLOR)
    h, w = image.shape[:2]
    assert 100 < w < 640 and 100 < h < 480


def test_nothing_happens_while_the_camera_warms_up():
    rig = Rig()
    rig.cfg.warmup_frames = 100
    rig.sorter._frames_seen = 0
    rig.show("empty", 0.5)
    rig.show("orange", 3.0)
    assert rig.agent.names() == []


def test_an_answer_for_an_item_no_longer_waited_on_is_ignored(rig: Rig):
    rig.show("empty", 1.2)
    rig.answer(decision(id="0199f000-0000-7000-8000-0000000000ff", action="discard", action_class="irreversible"))
    rig.show("empty", 0.5)
    assert rig.servo.sweeps == []  # e.g. an answer redelivered after a restart


def test_a_failing_agent_doesnt_stop_the_reflex(rig: Rig):
    def broken(*args, **kwargs):
        raise OSError("disk full")

    rig.agent.report_reflex = broken
    rig.show("empty", 1.2)
    rig.show("person", 0.5)
    assert rig.lit("red") and rig.sorter.person_present
