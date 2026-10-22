"""The agent's SQLite file: the journal and the queue in one table, so a reboot loses nothing."""

from __future__ import annotations

import json
import sqlite3
import threading
from pathlib import Path
from typing import Any

from .clock import now_iso
from .types import Correction, Decision

SCHEMA_VERSION = 1

_SCHEMA = """
CREATE TABLE IF NOT EXISTS schema_version (version INTEGER NOT NULL);

CREATE TABLE IF NOT EXISTS decisions (
    id                    TEXT PRIMARY KEY,
    revision              INTEGER NOT NULL,
    -- What the server has acknowledged. revision > synced_revision means "send me".
    synced_revision       INTEGER NOT NULL DEFAULT 0,
    attempts              INTEGER NOT NULL DEFAULT 0,
    frame_sent            INTEGER NOT NULL DEFAULT 0,
    -- 0 while an answer (or a give-up) of the server waits for the robot's on_decision to take it.
    delivered             INTEGER NOT NULL DEFAULT 1,

    -- Agent-owned facts.
    task                  TEXT NOT NULL,
    options               TEXT NOT NULL,
    action_class          TEXT NOT NULL,
    proposed_action       TEXT,
    prompt                TEXT,
    metadata              TEXT NOT NULL DEFAULT '{}',
    local_model           TEXT,
    local_confidence      REAL,
    local_latency_ms      INTEGER,
    needs_review          INTEGER NOT NULL DEFAULT 0,
    decided_at            TEXT NOT NULL,
    action                TEXT,
    executed_at           TEXT,
    cancelled_at          TEXT,
    cancel_reason         TEXT,
    correction_applied_at TEXT,

    -- Server-owned facts, and the local model's for a decision of the device.
    source                TEXT,
    model                 TEXT,
    confidence            REAL,
    reason                TEXT,
    latency_ms            INTEGER,
    input_tokens          INTEGER,
    output_tokens         INTEGER,
    cost_usd              REAL,
    answered_at           TEXT,
    received_at           TEXT,
    reviewed_at           TEXT,
    corrected_action      TEXT
);

CREATE TABLE IF NOT EXISTS frames (
    decision_id TEXT PRIMARY KEY REFERENCES decisions (id) ON DELETE CASCADE,
    jpeg        BLOB NOT NULL,
    width       INTEGER NOT NULL,
    height      INTEGER NOT NULL,
    created_at  TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS corrections (
    id          TEXT PRIMARY KEY,
    decision_id TEXT NOT NULL,
    action      TEXT NOT NULL,
    reason      TEXT NOT NULL,
    model       TEXT NOT NULL,
    created_at  TEXT NOT NULL,
    -- 0 until the robot's on_correction has taken it.
    notified    INTEGER NOT NULL DEFAULT 0
);

-- Waiting for the server: the robot asked, and only the server may answer.
CREATE INDEX IF NOT EXISTS decisions_waiting_idx ON decisions (id)
    WHERE action_class = 'irreversible' AND action IS NULL AND cancelled_at IS NULL;
"""

_WAITING = "action_class = 'irreversible' AND action IS NULL AND cancelled_at IS NULL"

# Fields the agent changes; every change bumps the revision.
AGENT_FIELDS = frozenset(
    {"action", "executed_at", "cancelled_at", "cancel_reason", "correction_applied_at", "attempts"}
)
# Fields only the server fills in.
SERVER_FIELDS = (
    "source", "model", "confidence", "reason", "latency_ms", "input_tokens", "output_tokens",
    "cost_usd", "answered_at", "received_at", "reviewed_at", "corrected_action",
)  # fmt: skip

_DECISION_COLUMNS = (
    "id", "revision", "task", "options", "action_class", "proposed_action", "prompt", "metadata",
    "local_model", "local_confidence", "local_latency_ms", "needs_review", "decided_at", "action",
    "executed_at", "cancelled_at", "cancel_reason", "correction_applied_at", "attempts",
    *SERVER_FIELDS,
)  # fmt: skip


def _decision(row: sqlite3.Row) -> Decision:
    values = {name: row[name] for name in _DECISION_COLUMNS}
    values["options"] = json.loads(values["options"])
    values["metadata"] = json.loads(values["metadata"])
    values["needs_review"] = bool(values["needs_review"])
    return Decision(**values)


class Store:
    def __init__(self, path: str | Path) -> None:
        self._lock = threading.RLock()
        # The robot's thread and the agent's background thread both write.
        self._db = sqlite3.connect(str(path), check_same_thread=False, isolation_level=None)
        self._db.row_factory = sqlite3.Row
        self._db.execute("PRAGMA journal_mode = WAL")
        self._db.execute("PRAGMA synchronous = NORMAL")
        self._db.execute("PRAGMA foreign_keys = ON")
        self._db.executescript(_SCHEMA)
        if self._db.execute("SELECT 1 FROM schema_version").fetchone() is None:
            self._db.execute("INSERT INTO schema_version VALUES (?)", (SCHEMA_VERSION,))

    def close(self) -> None:
        with self._lock:
            self._db.close()

    # --- reads ---------------------------------------------------------------

    def get(self, decision_id: str) -> Decision | None:
        with self._lock:
            row = self._db.execute("SELECT * FROM decisions WHERE id = ?", (decision_id,)).fetchone()
        return _decision(row) if row else None

    def recent(self, limit: int = 50) -> list[Decision]:
        with self._lock:
            rows = self._db.execute("SELECT * FROM decisions ORDER BY id DESC LIMIT ?", (limit,)).fetchall()
        return [_decision(r) for r in rows]

    def waiting(self, limit: int) -> list[tuple[Decision, bytes | None]]:
        """The oldest decisions only the server can make, with their frames."""
        with self._lock:
            rows = self._db.execute(
                f"SELECT * FROM decisions WHERE {_WAITING} ORDER BY id LIMIT ?", (limit,)
            ).fetchall()
            return [(_decision(r), self._frame(r["id"])) for r in rows]

    def pending_sync(self, limit: int) -> list[tuple[Decision, bytes | None]]:
        """Decisions the server hasn't seen in their current revision, oldest first. The frame is
        attached only the first time."""
        with self._lock:
            rows = self._db.execute(
                f"SELECT * FROM decisions WHERE revision > synced_revision AND NOT ({_WAITING}) ORDER BY id LIMIT ?",
                (limit,),
            ).fetchall()
            return [(_decision(r), None if r["frame_sent"] else self._frame(r["id"])) for r in rows]

    def count_pending_sync(self) -> int:
        with self._lock:
            return self._db.execute(
                f"SELECT count(*) FROM decisions WHERE revision > synced_revision AND NOT ({_WAITING})"
            ).fetchone()[0]

    def _frame(self, decision_id: str) -> bytes | None:
        row = self._db.execute("SELECT jpeg FROM frames WHERE decision_id = ?", (decision_id,)).fetchone()
        return bytes(row["jpeg"]) if row else None

    def frame(self, decision_id: str) -> bytes | None:
        with self._lock:
            return self._frame(decision_id)

    def correction(self, correction_id: str) -> Correction | None:
        with self._lock:
            row = self._db.execute("SELECT * FROM corrections WHERE id = ?", (correction_id,)).fetchone()
            if row is None:
                return None
            decision = self.get(row["decision_id"])
        return Correction(
            id=row["id"], decision_id=row["decision_id"], action=row["action"], reason=row["reason"],
            model=row["model"], created_at=row["created_at"],
            applied_at=decision.correction_applied_at if decision else None,
        )  # fmt: skip

    # --- writes --------------------------------------------------------------

    def insert(self, record: dict[str, Any], frame: tuple[bytes, int, int] | None) -> Decision:
        row = dict(record)
        row["options"] = json.dumps(row["options"])
        row["metadata"] = json.dumps(row["metadata"])
        row["needs_review"] = int(row["needs_review"])
        columns = ", ".join(row)
        marks = ", ".join("?" for _ in row)
        with self._lock:
            self._db.execute("BEGIN IMMEDIATE")
            try:
                self._db.execute(f"INSERT INTO decisions ({columns}) VALUES ({marks})", tuple(row.values()))
                if frame is not None:
                    jpeg, width, height = frame
                    self._db.execute(
                        "INSERT INTO frames (decision_id, jpeg, width, height, created_at) VALUES (?, ?, ?, ?, ?)",
                        (row["id"], jpeg, width, height, now_iso()),
                    )
                self._db.execute("COMMIT")
            except BaseException:
                self._db.execute("ROLLBACK")
                raise
        result = self.get(row["id"])
        assert result is not None
        return result

    def update_agent(
        self, decision_id: str, *, only_if_waiting: bool = False, undelivered: bool = False, **fields: Any
    ) -> Decision | None:
        """Change what the agent owns and bump the revision, so the next sync carries it. Returns None
        when the decision is missing or, with only_if_waiting, is no longer waiting. With undelivered, the
        robot still has to hear of the change through on_decision."""
        unknown = set(fields) - AGENT_FIELDS
        if unknown:
            raise ValueError(f"not an agent field: {', '.join(sorted(unknown))}")
        sets = ", ".join(f"{name} = ?" for name in fields)
        sql = f"UPDATE decisions SET {sets}{', ' if sets else ''}revision = revision + 1"
        if undelivered:
            sql += ", delivered = 0"
        sql += " WHERE id = ?"
        if only_if_waiting:
            sql += f" AND {_WAITING}"
        with self._lock:
            cur = self._db.execute(sql, (*fields.values(), decision_id))
            if cur.rowcount == 0:
                return None
            return self.get(decision_id)

    def count_attempt(self, decision_id: str) -> int:
        """A failed answer from the server. Doesn't touch the revision: it is the agent's bookkeeping."""
        with self._lock:
            self._db.execute("UPDATE decisions SET attempts = attempts + 1 WHERE id = ?", (decision_id,))
            row = self._db.execute("SELECT attempts FROM decisions WHERE id = ?", (decision_id,)).fetchone()
        return row["attempts"] if row else 0

    def apply_server(self, decision_id: str, server: dict[str, Any], sent_revision: int) -> Decision | None:
        """Take the server's fields of a decision and record that it holds the revision we sent.
        Never touches the agent's own fields: the robot may have changed them in the meantime."""
        with self._lock:
            current = self.get(decision_id)
            if current is None:
                return None
            fields = {k: server[k] for k in SERVER_FIELDS if k in server}
            # The action of an irreversible decision is the server's.
            answered = (
                current.action_class == "irreversible"
                and current.action is None
                and current.cancelled_at is None
                and bool(server.get("action"))
            )
            if current.action_class == "irreversible" and current.action is None and server.get("action"):
                fields["action"] = server["action"]
            sets = ", ".join(f"{name} = ?" for name in fields)
            self._db.execute(
                f"UPDATE decisions SET {sets}{', ' if sets else ''}"
                "synced_revision = max(synced_revision, ?), frame_sent = 1, "
                "delivered = CASE WHEN ? THEN 0 ELSE delivered END WHERE id = ?",
                (*fields.values(), sent_revision, int(answered), decision_id),
            )
            return self.get(decision_id)

    def undelivered(self) -> list[Decision]:
        """Answers and give-ups the robot hasn't heard of yet, unless it already acted on them."""
        with self._lock:
            rows = self._db.execute(
                "SELECT * FROM decisions WHERE delivered = 0 AND executed_at IS NULL ORDER BY id"
            ).fetchall()
        return [_decision(r) for r in rows]

    def mark_delivered(self, decision_id: str) -> None:
        with self._lock:
            self._db.execute("UPDATE decisions SET delivered = 1 WHERE id = ?", (decision_id,))

    def mark_synced(self, decision_id: str, revision: int) -> None:
        with self._lock:
            self._db.execute(
                "UPDATE decisions SET synced_revision = max(synced_revision, ?), frame_sent = 1 WHERE id = ?",
                (revision, decision_id),
            )

    def add_correction(self, c: Correction) -> bool:
        """Store a correction; False when it was already known."""
        with self._lock:
            cur = self._db.execute(
                "INSERT OR IGNORE INTO corrections (id, decision_id, action, reason, model, created_at) "
                "VALUES (?, ?, ?, ?, ?, ?)",
                (c.id, c.decision_id, c.action, c.reason, c.model, c.created_at),
            )
            if cur.rowcount == 0:
                return False
            self._db.execute(
                "UPDATE decisions SET corrected_action = ?, reviewed_at = coalesce(reviewed_at, ?) WHERE id = ?",
                (c.action, c.created_at, c.decision_id),
            )
            return True

    def unnotified_corrections(self) -> list[Correction]:
        with self._lock:
            rows = self._db.execute(
                "SELECT c.id FROM corrections c JOIN decisions d ON d.id = c.decision_id "
                "WHERE c.notified = 0 AND d.correction_applied_at IS NULL ORDER BY c.created_at"
            ).fetchall()
            found = [self.correction(r["id"]) for r in rows]
        return [c for c in found if c is not None]

    def mark_notified(self, correction_id: str) -> None:
        with self._lock:
            self._db.execute("UPDATE corrections SET notified = 1 WHERE id = ?", (correction_id,))

    def renotify_open_corrections(self) -> None:
        """After a restart the robot has forgotten the corrections it hasn't carried out: tell it again."""
        with self._lock:
            self._db.execute(
                "UPDATE corrections SET notified = 0 WHERE decision_id IN "
                "(SELECT id FROM decisions WHERE correction_applied_at IS NULL)"
            )

    def delete_old_frames(self, before: str) -> int:
        """Drop frames the server already has once they're older than `before`."""
        with self._lock:
            cur = self._db.execute(
                "DELETE FROM frames WHERE created_at < ? AND decision_id IN "
                "(SELECT id FROM decisions WHERE frame_sent = 1)",
                (before,),
            )
            return cur.rowcount
