from __future__ import annotations

from dataclasses import dataclass, field


@dataclass
class Config:
    # The part of the frame where an item is looked for, as fractions: x0, y0, x1, y1.
    zone: tuple[float, float, float, float] = (0.15, 0.15, 0.85, 0.95)
    # How long an item must sit in the zone before it becomes a task.
    dwell_s: float = 0.5
    # How long the zone must be empty before the next item counts as new.
    leave_s: float = 1.0
    # How long a person must be gone before the robot moves again.
    person_clear_s: float = 0.7
    person_confidence: float = 0.5
    # Below this a detection is noise, not a guess at the item's label.
    label_floor: float = 0.15
    # What the detector's labels mean for the rules: fruit goes to the basket.
    fruit_labels: frozenset[str] = frozenset({"orange", "apple", "banana"})
    # Frames to let the camera's exposure settle before the empty platform is learned.
    warmup_frames: int = 15
    # How long a light stays on after an action.
    hold_s: float = 2.5
    blink_period_s: float = 0.6
    # JPEG quality of the frames given to the agent, which shrinks them further.
    jpeg_quality: int = 90
    # The cut-out around the item: this much extra on every side, at least this many pixels wide.
    crop_margin: float = 0.4
    crop_min_px: int = 160
    # What the rules call things.
    task: str = "sort_item"
    reflex_task: str = "safety"
    reflex_action: str = "stop"
    pins: dict[str, int] = field(
        default_factory=lambda: {"green": 23, "yellow": 22, "red": 27, "blue": 24, "white": 25}
    )
    servo_pin: int = 12
    button_pin: int = 26
