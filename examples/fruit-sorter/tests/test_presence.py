from __future__ import annotations

import numpy as np

from fruit_sorter.presence import ZonePresence

ZONE = (0.0, 0.0, 1.0, 1.0)


def scene(level: int = 120) -> np.ndarray:
    frame = np.full((240, 320, 3), level, np.uint8)
    frame[:, :160] = level - 30  # not flat, like a real table
    return frame


def with_item(frame: np.ndarray, value: int = 250) -> np.ndarray:
    out = frame.copy()
    out[80:160, 120:200] = value
    return out


def test_an_item_is_found_with_its_box():
    zone = ZonePresence(ZONE)
    assert zone.update(scene()) is None  # learns the empty scene
    item = zone.update(with_item(scene()))
    assert item is not None and abs(item.box[0] - 120) <= 10 and abs(item.box[3] - 160) <= 10


def test_the_whole_scene_getting_lighter_or_darker_is_not_an_item():
    zone = ZonePresence(ZONE)
    zone.update(scene())
    for level in (135, 150, 105, 95):
        assert zone.update(scene(level)) is None


def test_an_item_is_still_found_when_the_light_changed_too():
    zone = ZonePresence(ZONE)
    zone.update(scene())
    assert zone.update(with_item(scene(140))) is not None


def test_reset_forgets_what_was_there():
    zone = ZonePresence(ZONE)
    zone.update(scene())
    assert zone.update(with_item(scene())) is not None
    zone.reset()
    zone.update(with_item(scene()))
    assert zone.update(with_item(scene())) is None


def test_taking_away_what_was_absorbed_is_a_removal_not_a_new_item():
    zone = ZonePresence(ZONE)
    zone.update(scene())
    assert zone.update(with_item(scene())) is not None
    zone.absorb()  # it stays, so it becomes part of the scene
    assert zone.update(with_item(scene())) is None
    assert zone.update(scene()) is None  # taken away: the hole is not an item
    assert zone.update(scene()) is None  # and the zone is back to normal
    assert zone.update(with_item(scene())) is not None  # a real item is still found afterwards


def test_a_different_item_where_the_absorbed_one_was_is_still_found():
    zone = ZonePresence(ZONE)
    zone.update(scene())
    zone.update(with_item(scene(), 250))
    zone.absorb()
    zone.update(with_item(scene(), 250))
    assert zone.update(with_item(scene(), 20)) is not None


def blob(x0: int, y0: int, x1: int, y1: int, value: int = 20) -> np.ndarray:
    out = scene().copy()
    out[y0:y1, x0:x1] = value
    return out


def test_a_shadow_along_the_edge_of_the_zone_is_not_an_item():
    zone = ZonePresence(ZONE)
    zone.update(scene())
    assert zone.update(blob(0, 20, 30, 200)) is None  # thin and touching the edge, like a shadow of a person


def test_a_blob_touching_the_edge_is_not_an_item_even_when_it_is_round():
    zone = ZonePresence(ZONE)
    zone.update(scene())
    assert zone.update(blob(0, 60, 90, 150)) is None


def test_a_thin_strip_inside_the_zone_is_not_an_item():
    zone = ZonePresence(ZONE)
    zone.update(scene())
    assert zone.update(blob(100, 40, 125, 200)) is None  # 25 wide, 160 tall


def test_a_change_covering_nearly_the_whole_zone_is_not_an_item():
    zone = ZonePresence(ZONE)
    zone.update(scene())
    assert zone.update(blob(8, 8, 312, 232)) is None  # inside the edges, but 94% of the zone


def test_an_item_is_still_found_beside_a_shadow():
    zone = ZonePresence(ZONE)
    zone.update(scene())
    frame = with_item(blob(0, 20, 30, 200))
    item = zone.update(frame)
    assert item is not None and 100 <= item.box[0] <= 130 and item.box[2] <= 210
