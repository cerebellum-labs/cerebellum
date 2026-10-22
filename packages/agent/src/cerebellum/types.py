from __future__ import annotations

from dataclasses import dataclass, field
from typing import Any


@dataclass(frozen=True)
class Decision:
    """A snapshot of a decision. Never changes; ask the agent again for a newer one."""

    id: str
    task: str
    options: list[str]
    action_class: str
    decided_at: str
    revision: int = 1
    proposed_action: str | None = None
    prompt: str | None = None
    metadata: dict[str, Any] = field(default_factory=dict)
    local_model: str | None = None
    local_confidence: float | None = None
    local_latency_ms: int | None = None
    needs_review: bool = False
    action: str | None = None
    executed_at: str | None = None
    cancelled_at: str | None = None
    cancel_reason: str | None = None
    correction_applied_at: str | None = None

    # What the server (or the local model, for a device decision) says about the action.
    source: str | None = None
    model: str | None = None
    confidence: float | None = None
    reason: str | None = None
    latency_ms: int | None = None
    input_tokens: int | None = None
    output_tokens: int | None = None
    cost_usd: float | None = None
    answered_at: str | None = None
    received_at: str | None = None
    reviewed_at: str | None = None
    corrected_action: str | None = None

    attempts: int = 0

    @property
    def status(self) -> str:
        """Derived from the facts exactly as the server does, so both sides always agree."""
        if self.cancelled_at is not None:
            return "cancelled"
        if self.corrected_action is not None:
            return "corrected"
        if self.executed_at is not None:
            return "executed"
        if self.action is None:
            return "waiting_for_server"
        if self.action_class == "irreversible":
            return "decided_on_server"
        return "decided_on_device"


@dataclass(frozen=True)
class Correction:
    """The server's review disagreed with the device: do `action` instead of the decision's one."""

    id: str
    decision_id: str
    action: str
    reason: str
    model: str
    created_at: str
    applied_at: str | None = None
