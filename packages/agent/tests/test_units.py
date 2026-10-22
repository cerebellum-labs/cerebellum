from __future__ import annotations

import io
import uuid

import pytest
from conftest import RULES, jpeg
from PIL import Image

from cerebellum import RulesError, frames
from cerebellum.clock import uuid7
from cerebellum.rules import Rules


def test_uuid7_is_version_7_and_sorts_by_creation():
    ids = [uuid7() for _ in range(5000)]
    assert all(uuid.UUID(i).version == 7 for i in ids)
    assert ids == sorted(ids) and len(set(ids)) == len(ids)


@pytest.mark.parametrize(
    "mutate, message",
    [
        (lambda r: r["tasks"]["sort_item"].pop("prompt"), "prompt"),
        (lambda r: r["tasks"]["sort_item"].update(options=[]), "options"),
        (lambda r: r["tasks"]["sort_item"].update(options=["basket", "basket"]), "distinct"),
        (lambda r: r["actions"]["discard"].update({"class": "maybe"}), "class"),
        (lambda r: r["actions"]["discard"].update(review_below=0.5), "reversible"),
        (lambda r: r["actions"]["basket"].update(review_below=1.5), "number in"),
        (lambda r: r["actions"].pop("discard"), "no class"),
    ],
)
def test_bad_rules_are_refused(mutate, message):
    import copy

    rules = copy.deepcopy(RULES)
    mutate(rules)
    with pytest.raises(RulesError, match=message):
        Rules.parse(rules)


def test_good_rules_parse():
    rules = Rules.parse(RULES)
    assert rules.actions["basket"].review_below == 0.8
    assert rules.tasks["sort_item"].options == ("basket", "discard")


def test_big_frames_shrink_and_small_ones_pass_untouched():
    big = jpeg(1920, 1080)
    out, w, h = frames.prepare(big, (640, 480), 80)
    assert w <= 640 and h <= 480 and len(out) < len(big)
    assert Image.open(io.BytesIO(out)).format == "JPEG"

    small = jpeg(320, 240)
    assert frames.prepare(small, (640, 480), 80) == (small, 320, 240)

    with pytest.raises(RulesError):
        frames.prepare(b"not an image", (640, 480), 80)
