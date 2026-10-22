"""Timestamps and ids. Times are ISO 8601 in UTC with milliseconds and a Z, so they sort as text."""

import os
import threading
import time
import uuid
from datetime import UTC, datetime, timedelta


def now_iso() -> str:
    return datetime.now(UTC).strftime("%Y-%m-%dT%H:%M:%S.%f")[:-3] + "Z"


def days_ago_iso(days: float) -> str:
    return (datetime.now(UTC) - timedelta(days=days)).strftime("%Y-%m-%dT%H:%M:%S.%f")[:-3] + "Z"


_lock = threading.Lock()
_last_ms = 0
_counter = 0


def uuid7() -> str:
    """A UUIDv7 that grows with every call, so ids sort by creation time even within a millisecond."""
    global _last_ms, _counter
    with _lock:
        ms = time.time_ns() // 1_000_000
        if ms <= _last_ms:
            ms = _last_ms
            _counter += 1
            if _counter >= 1 << 12:
                ms += 1
                _counter = 0
        else:
            _counter = int.from_bytes(os.urandom(2), "big") & 0x3FF
        _last_ms = ms
        counter = _counter
    rand_b = int.from_bytes(os.urandom(8), "big") & ((1 << 62) - 1)
    value = (ms << 80) | (0x7 << 76) | (counter << 64) | (0b10 << 62) | rand_b
    return str(uuid.UUID(int=value))
