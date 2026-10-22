"""Lights, servo and button: small interfaces the logic talks to. On a Mac they print, on the Pi they drive pins."""

from __future__ import annotations

import sys
import threading
import time
from collections.abc import Callable
from typing import Protocol

LIGHTS = ("green", "yellow", "red", "blue", "white")


class LightDriver(Protocol):
    def write(self, name: str, on: bool) -> None: ...


class NullDriver:
    def write(self, name: str, on: bool) -> None:
        pass


class GpioLights:
    """One LED per pin, through gpiozero (BCM numbers)."""

    def __init__(self, pins: dict[str, int]) -> None:
        from gpiozero import LED

        self._leds = {name: LED(pin) for name, pin in pins.items()}

    def write(self, name: str, on: bool) -> None:
        led = self._leds[name]
        led.on() if on else led.off()


class LightPanel:
    """What each light should be doing: off, on or blinking, or on for a moment and then back to what it was."""

    def __init__(
        self,
        driver: LightDriver,
        blink_period_s: float = 0.6,
        announce: Callable[[str, str], None] | None = None,
    ) -> None:
        self._driver = driver
        self._period = blink_period_s
        self._announce = announce
        self._modes = dict.fromkeys(LIGHTS, "off")
        self._pulse_until: dict[str, float] = {}
        self._written: dict[str, bool] = {}

    def set(self, name: str, mode: str) -> None:
        if mode not in ("off", "on", "blink"):
            raise ValueError(f"unknown mode {mode!r}")
        if self._modes[name] != mode:
            self._modes[name] = mode
            if self._announce:
                self._announce(name, {"off": "off", "on": "on", "blink": "blinking"}[mode])

    def pulse(self, name: str, seconds: float, now: float) -> None:
        self._pulse_until[name] = now + seconds
        if self._announce:
            self._announce(name, f"on for {seconds:g}s")

    def mode(self, name: str) -> str:
        return self._modes[name]

    def is_on(self, name: str, now: float) -> bool:
        if self._pulse_until.get(name, 0) > now:
            return True
        mode = self._modes[name]
        if mode == "blink":
            return int(now / (self._period / 2)) % 2 == 0
        return mode == "on"

    def tick(self, now: float) -> None:
        for name in LIGHTS:
            on = self.is_on(name, now)
            if self._written.get(name) != on:
                self._written[name] = on
                self._driver.write(name, on)

    def off(self) -> None:
        for name in LIGHTS:
            self._modes[name] = "off"
        self._pulse_until.clear()
        self.tick(time.monotonic())


class Actuator(Protocol):
    def sweep(self, action: str) -> None: ...


class ConsoleActuator:
    def __init__(self, announce: Callable[[str], None] | None = None) -> None:
        self._announce = announce or (lambda text: print(text, flush=True))

    def sweep(self, action: str) -> None:
        self._announce(f"servo: {'left, into the basket' if action == 'basket' else 'right, into the bin'}")


class ServoActuator:
    """The lever: left sweeps an item into the basket, right drops it off the edge into the bin."""

    def __init__(self, pin: int, left: float = -50, right: float = 50, rest: float = 0) -> None:
        from gpiozero import AngularServo

        self._servo = AngularServo(pin, min_angle=-90, max_angle=90, initial_angle=rest)
        self._angles = {"basket": left, "discard": right}
        self._rest = rest
        self._busy = threading.Lock()

    def sweep(self, action: str) -> None:
        # In a thread: the robot must keep looking for a hand while the lever moves.
        threading.Thread(target=self._sweep, args=(action,), daemon=True).start()

    def _sweep(self, action: str) -> None:
        with self._busy:
            self._servo.angle = self._angles[action]
            time.sleep(0.7)
            self._servo.angle = self._rest
            time.sleep(0.4)
            self._servo.detach()


class Button(Protocol):
    def pressed(self) -> bool:
        """True once per press."""
        ...


class GpioButton:
    def __init__(self, pin: int) -> None:
        from gpiozero import Button as GpioZeroButton

        self._presses = 0
        self._lock = threading.Lock()
        self._button = GpioZeroButton(pin, pull_up=True, bounce_time=0.05)
        self._button.when_pressed = self._on_press

    def _on_press(self) -> None:
        with self._lock:
            self._presses += 1

    def pressed(self) -> bool:
        with self._lock:
            if self._presses:
                self._presses -= 1
                return True
        return False


class ConsoleButton:
    """Enter on the keyboard is the button."""

    def __init__(self) -> None:
        self._presses = 0
        self._lock = threading.Lock()
        if sys.stdin and sys.stdin.isatty():
            threading.Thread(target=self._read, daemon=True).start()

    def _read(self) -> None:
        for _ in sys.stdin:
            self.press()

    def press(self) -> None:
        with self._lock:
            self._presses += 1

    def pressed(self) -> bool:
        with self._lock:
            if self._presses:
                self._presses -= 1
                return True
        return False
