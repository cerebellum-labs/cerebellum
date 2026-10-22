"""The user's rules: which tasks exist and which class each action belongs to."""

from __future__ import annotations

from dataclasses import dataclass
from typing import Any

from .errors import RulesError

CLASSES = ("reflex", "reversible", "irreversible")


@dataclass(frozen=True)
class Task:
    options: tuple[str, ...]
    prompt: str


@dataclass(frozen=True)
class ActionRule:
    action_class: str
    review_below: float | None = None


@dataclass(frozen=True)
class Rules:
    tasks: dict[str, Task]
    actions: dict[str, ActionRule]

    @classmethod
    def parse(cls, raw: dict[str, Any]) -> Rules:
        if not isinstance(raw, dict):
            raise RulesError("rules must be a dict with 'tasks' and 'actions'")
        tasks: dict[str, Task] = {}
        for name, spec in (raw.get("tasks") or {}).items():
            options = spec.get("options") if isinstance(spec, dict) else None
            prompt = spec.get("prompt") if isinstance(spec, dict) else None
            if not isinstance(options, list | tuple) or not options:
                raise RulesError(f"task {name!r} needs a non-empty 'options' list")
            if len(set(options)) != len(options) or not all(isinstance(o, str) and o for o in options):
                raise RulesError(f"task {name!r}: options must be distinct non-empty strings")
            if not isinstance(prompt, str) or not prompt.strip():
                raise RulesError(f"task {name!r} needs a 'prompt' for the cloud model")
            tasks[name] = Task(tuple(options), prompt)

        actions: dict[str, ActionRule] = {}
        for name, spec in (raw.get("actions") or {}).items():
            klass = spec.get("class") if isinstance(spec, dict) else None
            if klass not in CLASSES:
                raise RulesError(f"action {name!r} needs a 'class': one of {', '.join(CLASSES)}")
            review_below = spec.get("review_below")
            if review_below is not None:
                if klass != "reversible":
                    raise RulesError(f"action {name!r}: 'review_below' is only for reversible actions")
                if not isinstance(review_below, int | float) or not 0 < review_below <= 1:
                    raise RulesError(f"action {name!r}: 'review_below' must be a number in (0, 1]")
            actions[name] = ActionRule(klass, float(review_below) if review_below is not None else None)

        for name, task in tasks.items():
            unknown = [o for o in task.options if o not in actions]
            if unknown:
                raise RulesError(f"task {name!r} offers actions with no class: {', '.join(unknown)}")
        return cls(tasks, actions)
