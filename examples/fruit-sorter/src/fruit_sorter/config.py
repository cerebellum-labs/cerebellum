from __future__ import annotations

from dataclasses import dataclass, field


@dataclass
class Config:
    # The part of the frame where an item is looked for, as fractions: x0, y0, x1, y1.
    zone: tuple[float, float, float, float] = (0.15, 0.15, 0.85, 0.95)
    # How long an item must sit in the zone before it becomes a task.
    dwell_s: float = 0.5
    # How long the zone must be empty before the next item counts as new.
    leave_s: float = 0.6
    # How long something may sit in the zone, handled and with nothing to wait for, before the robot takes it for
    # part of the scene: whatever was left there, or a light that changed for good. Otherwise the zone would never
    # look empty again and no new item would ever count.
    settle_s: float = 6.0
    # How long a person must be gone before the robot moves again.
    person_clear_s: float = 0.7
    person_confidence: float = 0.5
    # Below this a detection is noise, not a guess at the item's label.
    label_floor: float = 0.15
    # What the detector's labels mean for the rules: fruit goes to the basket.
    fruit_labels: frozenset[str] = frozenset({"orange", "apple", "banana"})
    # Frames to let the camera's exposure settle before the empty platform is learned.
    warmup_frames: int = 15
    # An item in the zone counts as a new one, without the zone having been empty, when it sits somewhere else (less
    # overlap than this with the one handled) or looks different (average difference of its 32x32 grey picture, or of
    # its brightness, above these).
    swap_iou: float = 0.5
    swap_diff: float = 14.0
    swap_brightness: float = 45.0
    # How long a light stays on after an action.
    hold_s: float = 2.5
    blink_period_s: float = 0.6
    # JPEG quality of the frames given to the agent, which shrinks them further.
    jpeg_quality: int = 90
    # The cut-out around the item: this much extra on every side, at least this many pixels wide.
    crop_margin: float = 0.8
    crop_min_px: int = 320
    # What the rules call things.
    task: str = "sort_item"
    reflex_task: str = "safety"
    reflex_action: str = "stop"
    pins: dict[str, int] = field(
        default_factory=lambda: {"green": 23, "yellow": 22, "red": 27, "blue": 24, "white": 25}
    )
    servo_pin: int = 12
    button_pin: int = 26
