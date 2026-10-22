# Fruit sorter

A demo robot on a Raspberry Pi that uses Cerebellum. A camera looks at a platform; fruit goes into the basket, everything else into the bin, and lights show what the robot is doing. It is the example the project's demo video is shot on.

| Light | Means |
| --- | --- |
| Green | Into the basket |
| Blue | Thrown away |
| Yellow, steady | Waiting for the server |
| Yellow, blinking | Waiting for the server, and there is no network |
| Red | A person is in the frame, everything stops |
| White, blinking | A correction: take the item out of the basket and press the button |

How it decides, with the [Cerebellum agent](../../packages/agent):

1. The detector (YOLOX-Nano, trained on COCO) finds people and labels items. A person is a **reflex**: stop at once, one journal entry per appearance.
2. Whether something is in the zone is decided without the detector, by comparing with the empty platform, so a kiwi that COCO doesn't know still counts. An item that sits still for half a second becomes one task; the next one starts after the zone has been empty.
3. `orange`, `apple` and `banana` are proposed as `basket`, which is **reversible**: the agent accepts it on the spot, and the server reviews it later if the detector was below 0.8 sure. Everything else is proposed as `discard`, which is **irreversible**: only the server decides, the yellow light shows the wait, and without a network it keeps waiting.

## Run

Python 3.11 or newer and [uv](https://docs.astral.sh/uv/).

```bash
uv sync                          # on the Pi: uv sync --extra pi
uv run fruit-sorter download-model
cp .env.example .env             # the server's URL and this device's token, from the dashboard
```

Without any hardware, the demo's items on a made-up camera, with the real agent and server:

```bash
uv run fruit-sorter sim
```

With a camera; lights and the servo are printed unless `--pi` is given:

```bash
uv run fruit-sorter run
uv run fruit-sorter run --pi        # on the Raspberry Pi
```

The camera is found by itself (`--camera auto`, the default) and again after it is unplugged and plugged back, whatever number the system gives it; `--camera 1` or a video file pins one. `--zone x0,y0,x1,y1` sets the part of the frame (fractions) where items are looked for.

## Run at boot on the Pi

[`fruit-sorter.service`](fruit-sorter.service) starts the sorter when the Pi powers up and restarts it if it dies. The device token lives in `.env`, so keep it private (`chmod 600 .env`). Edit the user and paths in the unit, then:

```bash
sudo cp fruit-sorter.service /etc/systemd/system/
sudo systemctl daemon-reload && sudo systemctl enable --now fruit-sorter
journalctl -u fruit-sorter -f
```

The agent keeps its journal and queue in SQLite, so a power cut loses nothing: decisions made offline are sent after the next start.

## Wiring

| What | Pin on the Pi |
| --- | --- |
| Green | 16, GPIO23 |
| Yellow | 15, GPIO22 |
| Red | 13, GPIO27 |
| Blue | 18, GPIO24 |
| White | 22, GPIO25 |
| LEDs' common minus | 14, GND |
| Servo signal | 32, GPIO12 |
| Servo plus | 2, 5V |
| Servo minus | 6, GND |
| Button | 37, GPIO26 and 39, GND |

Each LED goes through a 220 Ω resistor; the button needs none, the Pi's internal pull-up is used.

## Develop

```bash
uv run pytest        # the logic on a made-up world, no camera or pins
uv run ruff check . && uv run ruff format --check .
```

The model is not in the repository; `download-model` fetches it to `models/` (see [THIRD_PARTY_NOTICES.md](../../THIRD_PARTY_NOTICES.md)).
