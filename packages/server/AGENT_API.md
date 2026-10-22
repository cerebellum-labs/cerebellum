# Agent API

How the agent on a robot talks to the Cerebellum server: the routes, what goes in and out, the errors, and how the agent should run its loop.

Every request starts on the device; the server never calls it, so a robot behind NAT or a phone hotspot works. Whatever the server has for the device comes back in a response: answers in `/decide`, corrections in `/sync`.

## Base URL

| Where | URL |
| --- | --- |
| Demo deployment | `https://api.cerebellumlabs.dev` |
| Local development (`make db run` in `packages/server`) | `http://localhost:8080` |

On the device both come from the environment:

```bash
CEREBELLUM_URL=https://api.cerebellumlabs.dev
CEREBELLUM_TOKEN=<the device token>
```

## Authentication

1. In the dashboard, open **Devices → Add device**, sign in with the dashboard password and give the device a name and a key like `fruit-sorter-1`.
2. The dashboard shows the device token once. Put it on the robot as `CEREBELLUM_TOKEN`; the server keeps only its SHA-256.
3. Send it with every request: `Authorization: Bearer <token>`.

**New token** in the dashboard replaces the token at once; **Delete** turns it off for good. Either way the old token gets 401. The device a request acts for always comes from the token, never from the body.

## Conventions

- Requests are `POST` with a JSON body, `Content-Type: application/json`, UTF-8. Responses are JSON.
- Times are ISO 8601 in UTC: `2026-10-21T18:09:00.123Z`. The server sends them with milliseconds and always with `Z`.
- Decision ids are UUIDv7, made by the agent: they sort by creation time, and a retried or offline-made decision is never stored twice.
- Every response has an `X-Request-ID` header. Send your own (letters, digits, `-`, `_`, `.`, up to 64) to find a request in the server's logs.

### Limits

| What | Limit |
| --- | --- |
| Request body | 5 MB |
| Items in `/decide` | 1 to 5 |
| Items in `/sync` | 0 to 10 |
| Frame | JPEG, at most 1 MB before base64 (640×480 at quality 80 is 30–60 KB) |
| `options` | 1 to 32, each 1 to 200 characters, no repeats |
| `task` | 1 to 200 characters |
| `prompt` | at most 4 KB |
| `metadata`, `device_metadata` | a JSON object, at most 16 KB |
| `agent_version` | at most 64 characters |

## Errors

### The whole request

Any status other than 200 means nothing was done. The body is:

```json
{"error": {"code": "unauthorized", "message": "the device token is wrong, was rotated, or its device was deleted"}}
```

| Status | `code` | Why | What the agent does |
| --- | --- | --- | --- |
| 400 | `bad_request` | Malformed JSON, too many items, bad `agent_version` or `device_metadata` | Logs it; doesn't send the same request again |
| 401 | `unauthorized` | No token, a wrong one, rotated, or the device deleted | Stops sending and logs it |
| 404 | `not_found` | Wrong path | Fix the URL |
| 405 | `method_not_allowed` | Not `POST` | Fix the method |
| 413 | `payload_too_large` | Body over 5 MB | Sends fewer items per request |
| 500 | `internal_error` | The server failed | Offline: tries again on the next round |
| — | — | Timeout or no network | Offline: tries again on the next round |

### One item

A 200 response can still turn some items down, each in `errors`, while the others go through:

```json
{"decision_id": "0199f000-...", "error": "options must have 1 to 32 entries", "retry": false}
```

- `retry: false`: the server will never accept the item as it is, a broken rule. Don't send it again unchanged.
- `retry: true`: it may work later, like a cloud model that gave no answer. For `/decide` this is a failed attempt.

## The decision the agent sends

The agent always sends a decision whole. The server keeps the parts the agent owns.

| Field | Type | | Rules |
| --- | --- | --- | --- |
| `id` | UUIDv7 | required | Made by the agent |
| `revision` | integer | required | 1 or more; bumped on every change the agent makes. The server applies only a higher one, so a late retry can't roll a decision back |
| `task` | string | required | Like `sort_item` |
| `options` | string[] | required | What the decision picks from. A reflex has its own action as the only option |
| `action_class` | string | required | `reflex`, `reversible` or `irreversible` |
| `proposed_action` | string | optional | What the robot wanted to do; one of the options |
| `prompt` | string | see rules | The question for the cloud model. Required for `irreversible` and with `needs_review` |
| `metadata` | object | optional | The team's own context: raw detector output, bounding box, servo angle. Shown in the dashboard and given to the model |
| `local_model` | string | optional | Like `yolox-nano` |
| `local_confidence` | number | optional | 0 to 1 |
| `needs_review` | boolean | optional | Only for `reversible` with a `prompt`: the server asks the cloud model later |
| `decided_at` | time | required | The robot's clock |
| `action` | string | see rules | The device's decision, one of the options. Required for `reflex` and `reversible`; for `irreversible` the server decides and ignores it |
| `executed_at` | time | see rules | When the robot acted. Required for a `reflex` |
| `cancelled_at` | time | optional | `irreversible` only, with `cancel_reason`, when the robot gave up waiting |
| `cancel_reason` | string | optional | Like "The item left the zone before the server answered." |
| `correction_applied_at` | time | optional | When the robot carried out the correction |

Once the server has a decision, later versions change only `revision`, `executed_at`, `cancelled_at`, `cancel_reason` and `correction_applied_at`. The request fields and the device's action never change.

A frame goes next to its decision as base64 JPEG: `{"decision": {...}, "frame": "/9j/4AAQ..."}`. The server keeps the first frame of a decision.

## The decision the server returns

The agent's fields come back as stored, plus what the server knows:

| Field | Type | What |
| --- | --- | --- |
| `status` | string | `decided_on_device`, `waiting_for_server`, `decided_on_server`, `executed`, `corrected` or `cancelled`. Computed from the facts below |
| `source` | string or null | `device` or `server`: who made `action`. Null while the server hasn't answered |
| `action` | string or null | The decision: the device's, or the server's for `irreversible` |
| `model` | string or null | The model behind `action`: the local one, or the cloud one for a server decision |
| `confidence` | number or null | That model's confidence, 0 to 1 |
| `reason` | string or null | The cloud model's explanation of a server decision |
| `latency_ms`, `input_tokens`, `output_tokens` | integer or null | The cloud call of a server decision |
| `cost_usd` | number or null | Its cost |
| `answered_at` | time or null | When the cloud model answered |
| `received_at` | time | When the server first got the decision, by its clock |
| `reviewed_at` | time or null | When the review finished |
| `corrected_action` | string or null | The review's answer when it disagreed with the device |
| `has_frame` | boolean | Whether the server has the frame |
| `device_id`, `device_key`, `device_name` | | The device |

The agent copies only these server fields into its own record and keeps its own: the robot may have changed them while the request was on its way.

## POST /api/v1/decide

Asks the cloud model for irreversible decisions, the ones only the server may make.

**Request:** 1 to 5 items, each an `irreversible` decision with a `prompt`.

```json
{
  "items": [{
    "decision": {
      "id": "0199f000-0000-7000-8000-000000000001",
      "revision": 1,
      "task": "sort_item",
      "options": ["basket", "discard"],
      "action_class": "irreversible",
      "proposed_action": "discard",
      "prompt": "Is the object in the photo a fruit? Answer basket if it is a fruit, discard otherwise.",
      "metadata": {"detector": {"label": "cup", "confidence": 0.88}, "bbox": [120, 80, 260, 210]},
      "local_model": "yolox-nano",
      "local_confidence": 0.88,
      "decided_at": "2026-10-21T18:09:00.123Z"
    },
    "frame": "/9j/4AAQSkZJRgABAQ..."
  }]
}
```

**Response 200:**

```json
{
  "decisions": [{
    "id": "0199f000-0000-7000-8000-000000000001",
    "status": "decided_on_server",
    "source": "server",
    "action": "discard",
    "model": "nvidia/Nemotron-3.5-Super-VL",
    "confidence": 0.97,
    "reason": "A plastic cup, not a fruit.",
    "latency_ms": 1820,
    "cost_usd": 0.000388,
    "answered_at": "2026-10-21T18:09:02.010Z",
    "...": "the other fields of the decision"
  }],
  "errors": []
}
```

- The items are answered in parallel. Each comes back either in `decisions` or in `errors`, in any order: match them by `id`.
- The model gets the prompt, the options, `metadata` as context and the frame. A reply that isn't one of the options is asked again once; every call, failed ones included, counts toward the dashboard's cost.
- **Retrying is safe.** A decision is asked once: if the agent stops waiting and sends it again, the second request waits for the call already running and gets its answer, and an answered decision comes back at once without a new call.
- An answer usually takes 2–10 s; the server gives a call up to 60 s.

## POST /api/v1/sync

Hands over every other change and brings back corrections. It is also the device's heartbeat: a device synced in the last 15 seconds shows as online.

**Request:** 0 to 10 items: new decisions and changed ones (executed, cancelled, a correction carried out).

```json
{
  "agent_version": "0.1.0",
  "device_metadata": {"camera": "usb-640x480", "board": "Raspberry Pi 5"},
  "items": [{
    "decision": {
      "id": "0199f000-0000-7000-8000-000000000002",
      "revision": 2,
      "task": "sort_item",
      "options": ["basket", "discard"],
      "action_class": "reversible",
      "proposed_action": "basket",
      "action": "basket",
      "prompt": "Is the object in the photo a fruit? Answer basket if it is a fruit, discard otherwise.",
      "needs_review": true,
      "local_model": "yolox-nano",
      "local_confidence": 0.63,
      "metadata": {"detector": {"label": "orange", "confidence": 0.63}},
      "decided_at": "2026-10-21T18:10:00.000Z",
      "executed_at": "2026-10-21T18:10:00.400Z"
    },
    "frame": "/9j/4AAQSkZJRgABAQ..."
  }]
}
```

**Response 200:**

```json
{
  "decisions": [{"id": "0199f000-0000-7000-8000-000000000002", "status": "executed", "revision": 2, "...": "..."}],
  "corrections": [{
    "id": "0199f000-0000-7000-8000-000000000002",
    "decision_id": "0199f000-0000-7000-8000-000000000002",
    "action": "discard",
    "reason": "A yellow tennis ball, not an orange.",
    "model": "nvidia/Nemotron-3.5-Super-VL",
    "created_at": "2026-10-21T18:10:04.200Z",
    "applied_at": null
  }],
  "errors": [],
  "server_time": "2026-10-21T18:10:05.000Z"
}
```

- `decisions` are the synced ones in the server's version; an older `revision` than the server's comes back as the server has it.
- `corrections` are all of the device's corrections not carried out yet, in every sync until they are. A decision has one correction at most, and it shares the decision's id.
- A decision with `needs_review` is reviewed in the background, usually within seconds. If the cloud model disagrees, the decision becomes `corrected` and its correction shows up in the next syncs.
- **To mark a correction carried out,** send the decision again with `correction_applied_at` and a higher `revision`.

## The agent's loop

What the server expects of the agent, from the spec:

- **Every 5 seconds**, and right away after the network comes back, the agent syncs. With nothing to send it still syncs, as the heartbeat and to fetch corrections.
- **Irreversible decisions** go to `/decide`, the oldest 5 first, with a 10-second timeout. An item with `retry: true`, a timeout, or no network counts as an attempt; after 3 the agent cancels the decision (`cancelled_at`, `cancel_reason`) and syncs it.
- **Everything else** goes to `/sync`, 10 at a time: decisions the server hasn't seen or that changed since the last sync. Compare revisions, not clocks: the robot's clock may be off. The frame goes only the first time a decision is synced.
- **Every change** bumps `revision` locally, and the next sync sends it.
- **New corrections** are matched by `id`: one already known is skipped.
- **On 401** the agent stops and logs it: the token needs replacing.

## Health

`GET /health` answers `{"status": "ok"}` while the server runs. It needs no token and doesn't check the database.
