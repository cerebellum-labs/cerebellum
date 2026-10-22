"""fruit-sorter run | sim | download-model"""

from __future__ import annotations

import argparse
import logging
import os
import signal
import sys
import threading
import time
import urllib.request
from pathlib import Path

from cerebellum import Cerebellum

from .camera import Camera
from .config import Config
from .detector import MODEL_URL, YoloxDetector
from .hardware import Actuator, Button, ConsoleActuator, ConsoleButton, LightPanel, NullDriver
from .presence import ZonePresence
from .sim import SimDetector, SimWorld, render
from .sorter import Sorter

DEFAULT_MODEL = Path(__file__).resolve().parents[2] / "models" / "yolox_nano.onnx"

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


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(prog="fruit-sorter", description=__doc__)
    sub = parser.add_subparsers(dest="command", required=True)

    run = sub.add_parser("run", help="sort items from a camera")
    run.add_argument(
        "--camera",
        default="auto",
        help="'auto' finds a working camera again after a replug; or an index, a file, a stream URL",
    )
    run.add_argument("--model", type=Path, default=DEFAULT_MODEL)
    run.add_argument("--pi", action="store_true", help="drive the lights, servo and button on the Raspberry Pi's pins")
    run.add_argument("--zone", help="x0,y0,x1,y1 as fractions of the frame")
    run.add_argument("--fps", type=float, default=10)

    sim = sub.add_parser("sim", help="play the demo's items on a made-up camera, with the real agent and server")
    sim.add_argument("--fast", action="store_true", help="shorter scenes")

    sub.add_parser("download-model", help=f"fetch the YOLOX-Nano weights (Apache-2.0) from {MODEL_URL}")

    for p in (run, sim):
        p.add_argument("--db", type=Path, help="the agent's SQLite file")
        p.add_argument("--env", type=Path, default=Path(".env"), help="file with CEREBELLUM_URL and CEREBELLUM_TOKEN")
    parser.add_argument("-v", "--verbose", action="store_true")

    args = parser.parse_args(argv)
    logging.basicConfig(
        level=logging.DEBUG if args.verbose else logging.INFO, format="%(asctime)s %(name)s %(message)s"
    )
    logging.getLogger("httpx").setLevel(logging.WARNING)

    if args.command == "download-model":
        return download_model(DEFAULT_MODEL)
    load_env(args.env)
    return {"run": cmd_run, "sim": cmd_sim}[args.command](args)


def load_env(path: Path) -> None:
    """KEY=VALUE lines; what is already in the environment wins."""
    if not path.is_file():
        return
    for line in path.read_text().splitlines():
        line = line.strip()
        if line and not line.startswith("#") and "=" in line:
            key, _, value = line.partition("=")
            os.environ.setdefault(key.strip(), value.strip().strip("\"'"))


def download_model(dest: Path) -> int:
    dest.parent.mkdir(parents=True, exist_ok=True)
    print(f"Downloading {MODEL_URL}\n  -> {dest}")
    urllib.request.urlretrieve(MODEL_URL, dest)  # noqa: S310 - a fixed https URL
    print(f"{dest.stat().st_size / 1e6:.1f} MB")
    return 0


def _announce(name: str, what: str) -> None:
    print(f"  light {name}: {what}", flush=True)


def _stopper() -> threading.Event:
    stop = threading.Event()
    for sig in (signal.SIGINT, signal.SIGTERM):
        signal.signal(sig, lambda *_: stop.set())
    return stop


def cmd_run(args: argparse.Namespace) -> int:
    cfg = Config()
    if args.zone:
        cfg.zone = tuple(float(v) for v in args.zone.split(","))  # type: ignore[assignment]
    if not args.model.is_file():
        print(f"No model at {args.model}: run `fruit-sorter download-model` first.", file=sys.stderr)
        return 2

    source = int(args.camera) if args.camera.isdigit() else args.camera  # "auto" stays as it is
    camera = Camera(source).start()
    if camera.read(timeout=5) is None:
        print(f"No frames from the camera {args.camera}.", file=sys.stderr)
        camera.close()
        return 2

    actuator: Actuator
    button: Button
    if args.pi:
        from .hardware import GpioButton, GpioLights, ServoActuator

        lights = LightPanel(GpioLights(cfg.pins), cfg.blink_period_s)
        actuator, button = ServoActuator(cfg.servo_pin), GpioButton(cfg.button_pin)
    else:
        lights = LightPanel(NullDriver(), cfg.blink_period_s, announce=_announce)
        actuator, button = ConsoleActuator(), ConsoleButton()

    agent = Cerebellum(RULES, db_path=args.db, device_metadata={"example": "fruit-sorter", "camera": f"{source}"})
    sorter = Sorter(agent, YoloxDetector(args.model), ZonePresence(cfg.zone), lights, actuator, button, cfg)
    sorter.attach()
    agent.start()
    stop = _stopper()
    print("Sorting. Ctrl+C to stop.", flush=True)
    period = 1 / args.fps
    try:
        while not stop.is_set():
            started = time.monotonic()
            frame = camera.read()
            if frame is not None:
                try:
                    sorter.step(frame)
                except Exception:
                    # One bad frame mustn't stop the robot.
                    logging.getLogger("fruit_sorter").exception("a step failed")
            time.sleep(max(0.0, period - (time.monotonic() - started)))
    finally:
        lights.off()
        camera.close()
        agent.stop()
    return 0


# What the made-up camera shows, in order: seconds and scene.
_DEMO = [
    (5, "empty"), (3, "orange"), (2.5, "empty"),            # a fruit the detector knows: green at once
    (6, "cup"), (3, "empty"),                               # a cup: yellow, the server confirms, blue
    (6, "kiwi"), (3, "empty"),                              # a kiwi COCO doesn't know: the server saves it
    (4, "uncertain_orange"), (8, "empty"),                  # a doubtful fruit: green, later the review disagrees
    (3, "person"), (2, "empty"),                            # a hand: red, everything stops
]  # fmt: skip


def cmd_sim(args: argparse.Namespace) -> int:
    cfg = Config()
    world = SimWorld()
    lights = LightPanel(NullDriver(), cfg.blink_period_s, announce=_announce)
    button = ConsoleButton()
    agent = Cerebellum(RULES, db_path=args.db, device_metadata={"example": "fruit-sorter", "camera": "simulated"})
    sorter = Sorter(agent, SimDetector(world), ZonePresence(cfg.zone), lights, ConsoleActuator(), button, cfg)
    sorter.attach()
    agent.start()
    stop = _stopper()
    scale = 0.6 if args.fast else 1
    pressing: set[str] = set()
    try:
        for seconds, scene in _DEMO:
            world.scene = scene
            print(f"--- {scene}", flush=True)
            frame = render(scene)
            until = time.monotonic() + seconds * scale
            while time.monotonic() < until and not stop.is_set():
                sorter.step(frame)
                # Stands in for the person who takes the item out of the basket and presses the button.
                for correction in sorter.corrections_pending:
                    if correction.id not in pressing:
                        pressing.add(correction.id)
                        threading.Timer(2.0, button.press).start()
                time.sleep(0.1)
            if stop.is_set():
                break
    finally:
        lights.off()
        agent.stop()
    return 0


if __name__ == "__main__":
    sys.exit(main())
