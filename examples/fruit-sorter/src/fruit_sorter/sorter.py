"""The robot's brain: from a camera frame to lights, the servo and calls to the Cerebellum agent."""

from __future__ import annotations

import logging
import queue
import time
from collections import deque
from collections.abc import Callable
from typing import Any, Protocol

import cv2
import numpy as np
from cerebellum import Correction, Decision, RulesError

from .config import Config
from .detector import Detection, Detector
from .hardware import Actuator, Button, LightPanel
from .presence import Item, ZonePresence

log = logging.getLogger("fruit_sorter")

BASKET = "basket"
DISCARD = "discard"


class Agent(Protocol):
    """The part of the Cerebellum agent the sorter uses."""

    @property
    def is_online(self) -> bool: ...
    def on_decision(self, fn: Callable[[Decision], None]) -> Any: ...
    def on_correction(self, fn: Callable[[Correction], None]) -> Any: ...
    def on_connection(self, fn: Callable[[bool], None]) -> Any: ...
    def decide(
        self,
        task: str,
        *,
        proposed_action: str | None = None,
        frame: bytes | None = None,
        local_model: str | None = None,
        local_confidence: float | None = None,
        local_latency_ms: int | None = None,
        metadata: dict[str, Any] | None = None,
    ) -> Decision: ...
    def report_reflex(
        self,
        task: str,
        action: str,
        *,
        frame: bytes | None = None,
        local_model: str | None = None,
        local_confidence: float | None = None,
        local_latency_ms: int | None = None,
        metadata: dict[str, Any] | None = None,
    ) -> Decision: ...
    def mark_executed(self, decision_id: str) -> Decision: ...
    def cancel(self, decision_id: str, reason: str = ...) -> Decision: ...
    def mark_correction_applied(self, correction_id: str) -> None: ...


class Sorter:
    def __init__(
        self,
        agent: Agent,
        detector: Detector,
        presence: ZonePresence,
        lights: LightPanel,
        actuator: Actuator,
        button: Button,
        config: Config | None = None,
    ) -> None:
        self.agent, self.detector, self.presence = agent, detector, presence
        self.lights, self.actuator, self.button = lights, actuator, button
        self.cfg = config or Config()

        # The agent calls back on its own thread; the robot's thread handles it here, so lights and
        # servo are only ever touched by one thread.
        self._events: queue.SimpleQueue[tuple[str, Any]] = queue.SimpleQueue()
        self.pending: Decision | None = None  # waiting for the server
        self._ready: deque[Decision] = deque()  # answered, to carry out once no person is in the way
        self._corrections: deque[Correction] = deque()

        self.person_present = False
        self._person_seen_at = 0.0
        self._armed = False  # the next item is a new one only after the zone was empty for a while
        self._empty_since: float | None = None
        self._item: Item | None = None
        self._item_since: float | None = None
        self._frames_seen = 0

    def attach(self) -> None:
        """Hook the sorter to the agent's callbacks; call before `agent.start()`."""
        self.agent.on_decision(lambda d: self._events.put(("decision", d)))
        self.agent.on_correction(lambda c: self._events.put(("correction", c)))
        self.agent.on_connection(lambda online: self._events.put(("connection", online)))

    @property
    def corrections_pending(self) -> list[Correction]:
        """Corrections the robot has been told of and a person hasn't carried out yet."""
        return list(self._corrections)

    # --- one tick of the main loop -------------------------------------------

    def step(self, frame: np.ndarray, now: float | None = None) -> None:
        now = time.monotonic() if now is None else now
        self._drain_events()

        started = time.perf_counter()
        detections = self.detector.detect(frame)
        latency_ms = max(1, int((time.perf_counter() - started) * 1000))

        self._watch_for_people(frame, detections, latency_ms, now)
        self._frames_seen += 1
        if self._frames_seen <= self.cfg.warmup_frames:
            # A camera's exposure settles in its first moments: what it saw before isn't the empty platform.
            if self._frames_seen == self.cfg.warmup_frames:
                self.presence.reset()
        elif not self.person_present:
            while self._ready:
                self._execute(self._ready.popleft(), now)
            self._watch_the_zone(frame, detections, latency_ms, now)
        self._watch_the_button()
        self._show_waiting()
        self.lights.tick(now)

    # --- the reflex ------------------------------------------------------------

    def _watch_for_people(self, frame: np.ndarray, detections: list[Detection], latency_ms: int, now: float) -> None:
        people = [d for d in detections if d.label == "person" and d.confidence >= self.cfg.person_confidence]
        if people:
            self._person_seen_at = now
            if not self.person_present:
                self.person_present = True
                self._item_since = None
                self.lights.set("red", "on")
                best = max(people, key=lambda d: d.confidence)
                # The stop doesn't wait for the journal: the red light is already on, and a failing agent
                # must not take the reflex down with it.
                try:
                    # One entry per appearance of a person, not per frame.
                    self.agent.report_reflex(
                        self.cfg.reflex_task,
                        self.cfg.reflex_action,
                        frame=self._jpeg(frame),
                        local_model=self.detector.name,
                        local_confidence=best.confidence,
                        local_latency_ms=latency_ms,
                        metadata={
                            "detector": {"label": best.label, "confidence": round(best.confidence, 3)},
                            "bbox": list(best.box),
                        },
                    )
                except Exception:
                    log.exception("could not journal the reflex")
        elif self.person_present and now - self._person_seen_at >= self.cfg.person_clear_s:
            self.person_present = False
            self.lights.set("red", "off")

    # --- the item ----------------------------------------------------------------

    def _watch_the_zone(self, frame: np.ndarray, detections: list[Detection], latency_ms: int, now: float) -> None:
        item = self.presence.update(frame)
        if item is None:
            self._item = self._item_since = None
            if self._empty_since is None:
                self._empty_since = now
            if now - self._empty_since >= self.cfg.leave_s:
                self._armed = True
            if self.pending is not None:
                self._item_left()
            return

        self._empty_since = None
        # Still for the whole dwell: a hand moving through the zone restarts the clock.
        if self._item is None or self._item_since is None or _iou(self._item.box, item.box) < 0.6:
            self._item_since = now
        self._item = item
        if self._armed and self.pending is None and now - self._item_since >= self.cfg.dwell_s:
            self._armed = False
            self._new_task(frame, item, detections, latency_ms, now)

    def _item_left(self) -> None:
        """The item went away while the server was still thinking."""
        assert self.pending is not None
        try:
            self.agent.cancel(self.pending.id, "The item left the zone before the server answered.")
        except RulesError:
            return  # the answer came in just now; its event is on its way
        self.pending = None

    def _new_task(
        self, frame: np.ndarray, item: Item, detections: list[Detection], latency_ms: int, now: float
    ) -> None:
        guess = self._guess(item, detections)
        proposed = BASKET if guess is not None and guess.label in self.cfg.fruit_labels else DISCARD
        crop, rect = self._cut_out(frame, item.box)
        decision = self.agent.decide(
            self.cfg.task,
            proposed_action=proposed,
            frame=crop,
            local_model=self.detector.name,
            local_confidence=guess.confidence if guess else None,
            local_latency_ms=latency_ms,
            metadata={
                "detector": {
                    "label": guess.label if guess else "unknown",
                    "confidence": round(guess.confidence, 3) if guess else None,
                    "top": [{"label": d.label, "confidence": round(d.confidence, 3)} for d in detections[:5]],
                },
                "bbox": list(item.box),
                "crop": rect,
            },
        )
        log.info("new item: %s -> %s (%s)", guess.label if guess else "unknown", proposed, decision.status)
        if decision.status == "waiting_for_server":
            self.pending = decision
        else:
            self._execute(decision, now)

    def _guess(self, item: Item, detections: list[Detection]) -> Detection | None:
        """The detector's best guess at what the item is, among what it saw inside the item."""
        x0, y0, x1, y1 = item.box
        margin_x, margin_y = (x1 - x0) * 0.1, (y1 - y0) * 0.1
        inside = [
            d
            for d in detections
            if d.label != "person"
            and d.confidence >= self.cfg.label_floor
            and x0 - margin_x <= d.center[0] <= x1 + margin_x
            and y0 - margin_y <= d.center[1] <= y1 + margin_y
        ]
        return max(inside, key=lambda d: d.confidence, default=None)

    def _cut_out(self, frame: np.ndarray, box: tuple[int, int, int, int]) -> tuple[bytes, list[int]]:
        height, width = frame.shape[:2]
        x0, y0, x1, y1 = box
        pad_x = max((x1 - x0) * self.cfg.crop_margin, (self.cfg.crop_min_px - (x1 - x0)) / 2)
        pad_y = max((y1 - y0) * self.cfg.crop_margin, (self.cfg.crop_min_px - (y1 - y0)) / 2)
        cx0, cy0 = max(0, int(x0 - pad_x)), max(0, int(y0 - pad_y))
        cx1, cy1 = min(width, int(x1 + pad_x)), min(height, int(y1 + pad_y))
        return self._jpeg(frame[cy0:cy1, cx0:cx1]), [cx0, cy0, cx1 - cx0, cy1 - cy0]

    def _jpeg(self, image: np.ndarray) -> bytes:
        ok, buf = cv2.imencode(".jpg", image, [cv2.IMWRITE_JPEG_QUALITY, self.cfg.jpeg_quality])
        if not ok:
            raise RuntimeError("could not encode a frame")
        return buf.tobytes()

    # --- what the agent tells us ---------------------------------------------------

    def _drain_events(self) -> None:
        while True:
            try:
                kind, value = self._events.get_nowait()
            except queue.Empty:
                return
            if kind == "decision":
                self._on_decision(value)
            elif kind == "correction":
                self._corrections.append(value)
                self.lights.set("white", "blink")
            # "connection" needs nothing here: _show_waiting reads agent.is_online every tick.

    def _on_decision(self, decision: Decision) -> None:
        if self.pending is None or self.pending.id != decision.id:
            # Not what the robot is waiting for now: an answer redelivered after a restart, or for an item long gone.
            log.info("ignoring the late %s decision %s", decision.status, decision.id)
            return
        self.pending = None
        if decision.status != "cancelled":
            self._ready.append(decision)

    def _execute(self, decision: Decision, now: float) -> None:
        if decision.action == BASKET:
            self.lights.pulse("green", self.cfg.hold_s, now)
        elif decision.action == DISCARD:
            self.lights.pulse("blue", self.cfg.hold_s, now)
        else:
            log.warning("no way to carry out %r", decision.action)
            return
        self.actuator.sweep(decision.action)
        self.agent.mark_executed(decision.id)

    def _show_waiting(self) -> None:
        if self.pending is None:
            self.lights.set("yellow", "off")
        else:
            # Steady: the server has it. Blinking: it will, when the network is back.
            self.lights.set("yellow", "on" if self.agent.is_online else "blink")

    def _watch_the_button(self) -> None:
        if self.button.pressed() and self._corrections:
            self.agent.mark_correction_applied(self._corrections.popleft().id)
            if not self._corrections:
                self.lights.set("white", "off")


def _iou(a: tuple[int, int, int, int], b: tuple[int, int, int, int]) -> float:
    ix = max(0, min(a[2], b[2]) - max(a[0], b[0]))
    iy = max(0, min(a[3], b[3]) - max(a[1], b[1]))
    inter = ix * iy
    union = (a[2] - a[0]) * (a[3] - a[1]) + (b[2] - b[0]) * (b[3] - b[1]) - inter
    return inter / union if union else 0.0
