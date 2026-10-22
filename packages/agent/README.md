# Cerebellum agent

The on-device library of Cerebellum, imported by the robot code. It decides by your rules, keeps a journal and a queue in SQLite that survive a reboot, and talks to the [server](../server) in the background. The wire format is [AGENT_API.md](../server/AGENT_API.md).

## Use

```python
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

agent = Cerebellum(rules=RULES, device_metadata={"camera": "usb-640x480"})  # CEREBELLUM_URL and CEREBELLUM_TOKEN
agent.on_decision(handle_decision)  # the server answered, or gave up
agent.on_correction(handle_correction)  # a review disagreed: fix the consequences
agent.on_connection(handle_connection)  # the network went away or came back
agent.start()

d = agent.decide(
    "sort_item",
    proposed_action="discard",
    frame=jpeg_bytes,
    local_model="yolox-nano",
    local_confidence=0.3,
    metadata={"bbox": [120, 80, 260, 210]},
)
# d.status is "decided_on_device" (a reversible proposal) or "waiting_for_server"; decide never blocks.
agent.report_reflex("safety", "stop", frame=jpeg_bytes)  # journal a reflex already done
agent.mark_executed(d.id)
agent.cancel(d.id)  # stop waiting for the server
agent.mark_correction_applied(correction.id)
```

Callbacks run on the agent's background thread.

| Class | Who decides | Offline |
| --- | --- | --- |
| reflex | the robot; `report_reflex` only journals it | works |
| reversible | the agent, from the local model's answer; uncertain ones (below `review_below`, or no confidence) are reviewed by the server later | works |
| irreversible | only the server; `decide` returns `waiting_for_server` and the answer comes to `on_decision` | waits, however long |

A decision is cancelled only for a real failure: the robot calls `cancel`, or the server says it can't decide (after 3 failed model calls, or at once when it rejects the decision). Having no network is never a failure: the decision just keeps waiting.

## Settings

Passed to the constructor; `server_url` and `token` default to `CEREBELLUM_URL` and `CEREBELLUM_TOKEN`.

| Parameter | Default |
| --- | --- |
| `db_path` | `cerebellum.db` in the user data directory (`~/.local/share/cerebellum` on Linux) |
| `sync_interval` | 5 s |
| `decide_timeout`, `sync_timeout` | 10 s, 15 s |
| `max_attempts` | 3 |
| `frame_size`, `frame_quality` | 640×480, JPEG 80: the agent shrinks frames itself |
| `decide_batch`, `sync_batch` | 5, 10 |
| `keep_frames_days` | 7: frames the server already has are deleted after that |
| `device_metadata` | `{}` |

## Develop

Python 3.11 or newer, [uv](https://docs.astral.sh/uv/).

```bash
uv sync
uv run pytest      # against a fake server that follows AGENT_API.md
uv run ruff check . && uv run ruff format --check .
```
