"""The agent: decides by the user's rules, keeps the journal, talks to the server in the background."""

from __future__ import annotations

import base64
import json
import logging
import os
import threading
from collections.abc import Callable
from pathlib import Path
from typing import Any

import platformdirs

from . import frames
from .client import Client, Offline, Rejected, Unauthorized
from .clock import days_ago_iso, now_iso, uuid7
from .errors import RulesError
from .rules import Rules
from .store import Store
from .types import Correction, Decision

log = logging.getLogger("cerebellum")

MAX_METADATA_BYTES = 16 << 10

# What one step of the background thread reports: more to do right now, nothing left, or the network/token failed.
_MORE, _DONE, _FAILED = "more", "done", "failed"
# A bound on steps in one round, so a misbehaving server can't keep the thread in a tight loop.
_MAX_STEPS = 50


def _agent_version() -> str:
    from . import __version__

    return __version__


class Cerebellum:
    def __init__(
        self,
        rules: dict[str, Any],
        *,
        server_url: str | None = None,
        token: str | None = None,
        db_path: str | Path | None = None,
        sync_interval: float = 5.0,
        decide_timeout: float = 10.0,
        sync_timeout: float = 15.0,
        max_attempts: int = 3,
        frame_size: tuple[int, int] = (640, 480),
        frame_quality: int = 80,
        decide_batch: int = 5,
        sync_batch: int = 10,
        keep_frames_days: float = 7,
        device_metadata: dict[str, Any] | None = None,
    ) -> None:
        self.rules = Rules.parse(rules)
        server_url = server_url or os.environ.get("CEREBELLUM_URL")
        token = token or os.environ.get("CEREBELLUM_TOKEN")
        if not server_url:
            raise ValueError("server_url or CEREBELLUM_URL is required")
        if not token:
            raise ValueError("token or CEREBELLUM_TOKEN is required")
        if db_path is None:
            db_path = Path(platformdirs.user_data_dir("cerebellum")) / "cerebellum.db"
        db_path = Path(db_path)
        db_path.parent.mkdir(parents=True, exist_ok=True)

        self.sync_interval = sync_interval
        self.decide_timeout = decide_timeout
        self.sync_timeout = sync_timeout
        self.max_attempts = max_attempts
        self.frame_size = frame_size
        self.frame_quality = frame_quality
        self.decide_batch = decide_batch
        self.sync_batch = sync_batch
        self.keep_frames_days = keep_frames_days
        self.device_metadata = device_metadata or {}
        self._check_json(self.device_metadata, "device_metadata")

        self._store = Store(db_path)
        self._store.renotify_open_corrections()
        self._client = Client(server_url, token, _agent_version())
        self._decision_cb: Callable[[Decision], None] | None = None
        self._correction_cb: Callable[[Correction], None] | None = None
        self._connection_cb: Callable[[bool], None] | None = None

        self._online = False
        self._auth_failed = False
        self._effective_sync_batch = sync_batch
        self._effective_decide_batch = decide_batch
        self._wake = threading.Event()
        self._stop = threading.Event()
        self._thread: threading.Thread | None = None

    # --- callbacks, called on the agent's thread -----------------------------

    def on_decision(self, fn: Callable[[Decision], None]) -> Callable[[Decision], None]:
        """The server answered a decision the robot was waiting for, or gave up on it."""
        self._decision_cb = fn
        return fn

    def on_correction(self, fn: Callable[[Correction], None]) -> Callable[[Correction], None]:
        """A review disagreed with a decision: the robot should fix its consequences."""
        self._correction_cb = fn
        return fn

    def on_connection(self, fn: Callable[[bool], None]) -> Callable[[bool], None]:
        """The network went away (False) or came back (True)."""
        self._connection_cb = fn
        return fn

    # --- state ---------------------------------------------------------------

    @property
    def is_online(self) -> bool:
        return self._online

    @property
    def auth_failed(self) -> bool:
        """The server rejected the token: replace CEREBELLUM_TOKEN. Nothing is sent until restart."""
        return self._auth_failed

    def get(self, decision_id: str) -> Decision | None:
        return self._store.get(decision_id)

    def recent(self, limit: int = 50) -> list[Decision]:
        return self._store.recent(limit)

    # --- life cycle ----------------------------------------------------------

    def start(self) -> None:
        if self._thread is not None:
            return
        self._thread = threading.Thread(target=self._run, name="cerebellum-agent", daemon=True)
        self._thread.start()

    def stop(self) -> None:
        self._stop.set()
        self._wake.set()
        if self._thread is not None:
            self._thread.join(timeout=self.sync_timeout + 5)
            self._thread = None
        self._client.close()
        self._store.close()

    def __enter__(self) -> Cerebellum:
        self.start()
        return self

    def __exit__(self, *exc: object) -> None:
        self.stop()

    # --- the robot's calls ---------------------------------------------------

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
    ) -> Decision:
        """Record a decision and return at once. A reversible proposal is accepted on the spot; anything
        else waits for the server, and the answer arrives in `on_decision`."""
        spec = self.rules.tasks.get(task)
        if spec is None:
            raise RulesError(f"unknown task {task!r}")
        klass = None
        if proposed_action is not None:
            if proposed_action not in spec.options:
                raise RulesError(f"{proposed_action!r} is not an option of task {task!r}")
            klass = self.rules.actions[proposed_action].action_class
            if klass == "reflex":
                raise RulesError("a reflex is not decided: report it with report_reflex")
        self._check_confidence(local_confidence)

        record = self._record(task, list(spec.options), local_model, local_confidence, local_latency_ms, metadata)
        record["prompt"] = spec.prompt
        record["proposed_action"] = proposed_action
        if klass == "reversible":
            assert proposed_action is not None
            threshold = self.rules.actions[proposed_action].review_below
            record.update(
                action_class="reversible",
                action=proposed_action,
                source="device",
                model=local_model,
                confidence=local_confidence,
                latency_ms=local_latency_ms,
                needs_review=local_confidence is None or (threshold is not None and local_confidence < threshold),
            )
        else:
            # Only the server may decide this, also when the robot had no proposal.
            record.update(action_class="irreversible", action=None)
        decision = self._store.insert(record, self._prepare(frame))
        self._wake.set()
        return decision

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
    ) -> Decision:
        """Journal a reflex the robot has already done."""
        rule = self.rules.actions.get(action)
        if rule is None or rule.action_class != "reflex":
            raise RulesError(f"{action!r} is not a reflex in the rules")
        self._check_confidence(local_confidence)
        record = self._record(task, [action], local_model, local_confidence, local_latency_ms, metadata)
        record.update(
            action_class="reflex",
            action=action,
            executed_at=record["decided_at"],
            source="device",
            model=local_model,
            confidence=local_confidence,
            latency_ms=local_latency_ms,
        )
        decision = self._store.insert(record, self._prepare(frame))
        self._wake.set()
        return decision

    def mark_executed(self, decision_id: str) -> Decision:
        decision = self._require(decision_id)
        if decision.executed_at is not None:
            return decision
        if decision.status not in ("decided_on_device", "decided_on_server"):
            raise RulesError(f"a decision that is {decision.status} can't be executed")
        updated = self._store.update_agent(decision_id, executed_at=now_iso())
        self._wake.set()
        assert updated is not None
        return updated

    def cancel(self, decision_id: str, reason: str = "The robot stopped waiting for the server.") -> Decision:
        decision = self._require(decision_id)
        if decision.status != "waiting_for_server":
            raise RulesError(f"only a waiting decision can be cancelled, this one is {decision.status}")
        updated = self._store.update_agent(
            decision_id, only_if_waiting=True, cancelled_at=now_iso(), cancel_reason=reason
        )
        self._wake.set()
        # The answer came in while we were deciding to cancel: it stands.
        return updated or self._require(decision_id)

    def mark_correction_applied(self, correction_id: str) -> None:
        correction = self._store.correction(correction_id)
        if correction is None:
            raise RulesError(f"unknown correction {correction_id!r}")
        if correction.applied_at is None:
            self._store.update_agent(correction.decision_id, correction_applied_at=now_iso())
            self._wake.set()

    # --- helpers for the calls -----------------------------------------------

    def _require(self, decision_id: str) -> Decision:
        decision = self._store.get(decision_id)
        if decision is None:
            raise RulesError(f"unknown decision {decision_id!r}")
        return decision

    @staticmethod
    def _check_confidence(value: float | None) -> None:
        if value is not None and not 0 <= value <= 1:
            raise RulesError("local_confidence must be between 0 and 1")

    @staticmethod
    def _check_json(value: dict[str, Any], name: str) -> None:
        try:
            encoded = json.dumps(value, allow_nan=False)
        except (TypeError, ValueError) as exc:
            raise RulesError(f"{name} must be JSON-serializable: {exc}") from exc
        if len(encoded.encode()) > MAX_METADATA_BYTES:
            raise RulesError(f"{name} must be at most 16 KB")

    def _record(
        self,
        task: str,
        options: list[str],
        local_model: str | None,
        local_confidence: float | None,
        local_latency_ms: int | None,
        metadata: dict[str, Any] | None,
    ) -> dict[str, Any]:
        metadata = metadata or {}
        self._check_json(metadata, "metadata")
        return {
            "id": uuid7(),
            "revision": 1,
            "task": task,
            "options": options,
            "action_class": "",
            "proposed_action": None,
            "prompt": None,
            "metadata": metadata,
            "local_model": local_model,
            "local_confidence": local_confidence,
            "local_latency_ms": local_latency_ms,
            "needs_review": False,
            "decided_at": now_iso(),
        }

    def _prepare(self, frame: bytes | None) -> tuple[bytes, int, int] | None:
        if frame is None:
            return None
        return frames.prepare(frame, self.frame_size, self.frame_quality)

    # --- the background thread -----------------------------------------------

    def _run(self) -> None:
        while not self._stop.is_set():
            self._wake.clear()
            try:
                self._cycle()
            except Exception:
                log.exception("sync round failed")
            self._wake.wait(self.sync_interval)

    def _cycle(self) -> None:
        if self._auth_failed:
            return
        self._deliver()
        # Irreversible decisions first: the robot is waiting for them.
        for _ in range(_MAX_STEPS):
            outcome = self._decide_step()
            if outcome != _MORE:
                break
        if outcome == _FAILED:
            return
        # Even with nothing to send, a sync is the heartbeat and fetches corrections.
        for _ in range(_MAX_STEPS):
            outcome = self._sync_step()
            if outcome != _MORE:
                break
        if outcome == _FAILED:
            return
        self._deliver()
        self._store.delete_old_frames(days_ago_iso(self.keep_frames_days))

    def _decide_step(self) -> str:
        waiting = self._store.waiting(self._effective_decide_batch)
        if not waiting:
            return _DONE
        sent = {d.id: d.revision for d, _ in waiting}
        body = {"items": [_item(d, f) for d, f in waiting]}
        try:
            resp = self._client.post("/api/v1/decide", body, self.decide_timeout)
        except Offline as exc:
            self._went_offline(exc)
            return _FAILED
        except Unauthorized as exc:
            self._unauthorized(exc)
            return _FAILED
        except Rejected as exc:
            if exc.status == 413 and len(waiting) > 1:
                self._effective_decide_batch = max(1, len(waiting) // 2)
                log.warning("decide batch too large, now %d", self._effective_decide_batch)
                return _MORE
            log.error("the server rejected /decide: %s", exc)
            for d, _ in waiting:
                self._give_up(d.id, f"The server rejected the request: {exc.message}")
            self._deliver()
            return _MORE
        self._set_online(True)

        for server in resp.get("decisions") or []:
            self._store.apply_server(server["id"], server, sent.get(server["id"], 0))
        errors = resp.get("errors") or []
        for err in errors:
            self._decide_failed(err)
        self._deliver()
        # Failed items wait for the next round instead of hammering the model again at once.
        return _MORE if len(waiting) == self._effective_decide_batch and not errors else _DONE

    def _decide_failed(self, err: dict[str, Any]) -> None:
        decision_id, message = err.get("decision_id", ""), err.get("error", "unknown error")
        decision = self._store.get(decision_id)
        if decision is None or decision.cancelled_at is not None:
            return
        if not err.get("retry", False):
            self._give_up(decision_id, f"The server can't decide this: {message}")
            return
        attempts = self._store.count_attempt(decision_id)
        log.warning(
            "the server couldn't decide %s (try %d of %d): %s", decision_id, attempts, self.max_attempts, message
        )
        if attempts >= self.max_attempts:
            self._give_up(decision_id, f"No answer from the server after {attempts} tries: {message}")

    def _give_up(self, decision_id: str, reason: str) -> None:
        self._store.update_agent(
            decision_id, only_if_waiting=True, undelivered=True, cancelled_at=now_iso(), cancel_reason=reason
        )

    def _sync_step(self) -> str:
        pending = self._store.pending_sync(self._effective_sync_batch)
        sent = {d.id: d.revision for d, _ in pending}
        body = {
            "agent_version": _agent_version(),
            "device_metadata": self.device_metadata,
            "items": [_item(d, f) for d, f in pending],
        }
        try:
            resp = self._client.post("/api/v1/sync", body, self.sync_timeout)
        except Offline as exc:
            self._went_offline(exc)
            return _FAILED
        except Unauthorized as exc:
            self._unauthorized(exc)
            return _FAILED
        except Rejected as exc:
            if exc.status == 413 and len(pending) > 1:
                self._effective_sync_batch = max(1, len(pending) // 2)
                log.warning("sync batch too large, now %d", self._effective_sync_batch)
                return _MORE
            # Not sending the same request again: it would be turned down again.
            log.error("the server rejected /sync, dropping %d item(s): %s", len(pending), exc)
            for decision_id, revision in sent.items():
                self._store.mark_synced(decision_id, revision)
            return _MORE if pending and self._store.count_pending_sync() > 0 else _DONE
        self._set_online(True)

        for server in resp.get("decisions") or []:
            self._store.apply_server(server["id"], server, sent.get(server["id"], 0))
        for err in resp.get("errors") or []:
            decision_id = err.get("decision_id", "")
            log.error("the server turned down decision %s: %s", decision_id, err.get("error"))
            if not err.get("retry", False) and decision_id in sent:
                self._store.mark_synced(decision_id, sent[decision_id])
        for raw in resp.get("corrections") or []:
            self._store.add_correction(_correction(raw))
        return _MORE if pending and self._store.count_pending_sync() > 0 else _DONE

    def _deliver(self) -> None:
        """Hand the robot what it hasn't heard of. A callback is called at least once: if the program dies
        between the server's answer and the callback, it is called again after the restart."""
        if self._decision_cb is not None:
            for decision in self._store.undelivered():
                self._call(self._decision_cb, decision)
                self._store.mark_delivered(decision.id)
        if self._correction_cb is not None:
            for correction in self._store.unnotified_corrections():
                self._call(self._correction_cb, correction)
                self._store.mark_notified(correction.id)

    # --- connection ----------------------------------------------------------

    def _set_online(self, online: bool) -> None:
        if online == self._online:
            return
        self._online = online
        log.info("the server is %s", "reachable" if online else "unreachable")
        self._call(self._connection_cb, online)

    def _went_offline(self, exc: Exception) -> None:
        log.warning("offline: %s", exc)
        self._set_online(False)

    def _unauthorized(self, exc: Exception) -> None:
        log.error("the server doesn't accept the token (%s): replace CEREBELLUM_TOKEN", exc)
        self._auth_failed = True
        self._set_online(False)

    @staticmethod
    def _call(callback: Callable[[Any], None] | None, arg: Any) -> None:
        if callback is None:
            return
        try:
            callback(arg)
        except Exception:
            log.exception("callback %s failed", getattr(callback, "__name__", callback))


def _correction(raw: dict[str, Any]) -> Correction:
    return Correction(
        id=raw["id"],
        decision_id=raw["decision_id"],
        action=raw["action"],
        reason=raw.get("reason", ""),
        model=raw.get("model", ""),
        created_at=raw["created_at"],
        applied_at=raw.get("applied_at"),
    )


def _item(decision: Decision, frame: bytes | None) -> dict[str, Any]:
    """A decision as the server takes it: the agent's facts only."""
    d = decision
    return {
        "decision": {
            "id": d.id,
            "revision": d.revision,
            "task": d.task,
            "options": d.options,
            "action_class": d.action_class,
            "proposed_action": d.proposed_action,
            "prompt": d.prompt,
            "metadata": d.metadata,
            "local_model": d.local_model,
            "local_confidence": d.local_confidence,
            "local_latency_ms": d.local_latency_ms,
            "needs_review": d.needs_review,
            "decided_at": d.decided_at,
            # For an irreversible decision the server decides and ignores this.
            "action": None if d.action_class == "irreversible" else d.action,
            "executed_at": d.executed_at,
            "cancelled_at": d.cancelled_at,
            "cancel_reason": d.cancel_reason,
            "correction_applied_at": d.correction_applied_at,
        },
        "frame": base64.b64encode(frame).decode() if frame else None,
    }
