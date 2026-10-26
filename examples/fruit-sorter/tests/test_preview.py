import urllib.error
import urllib.request

import numpy as np

from fruit_sorter.detector import Detection
from fruit_sorter.presence import ZonePresence
from fruit_sorter.preview import Preview, Previewed


def test_serves_the_newest_frame_with_the_zone_drawn_on_it():
    preview = Preview(ZonePresence((0.2, 0.2, 0.8, 0.8)), port=0, host="127.0.0.1")
    try:
        preview.show(np.zeros((120, 160, 3), np.uint8), [Detection("apple", 0.9, (40, 40, 80, 80))])
        with urllib.request.urlopen(f"http://127.0.0.1:{preview.port}/snapshot.jpg", timeout=5) as response:
            body = response.read()
            assert response.headers["Content-Type"] == "image/jpeg"
        assert body.startswith(b"\xff\xd8")
    finally:
        preview.close()


def test_no_frame_yet_is_not_an_error_page():
    preview = Preview(ZonePresence((0, 0, 1, 1)), port=0, host="127.0.0.1")
    try:
        try:
            urllib.request.urlopen(f"http://127.0.0.1:{preview.port}/snapshot.jpg", timeout=5)
        except urllib.error.HTTPError as e:
            assert e.code == 503
    finally:
        preview.close()


def test_a_previewed_detector_still_answers_with_what_it_found():
    class Inner:
        name = "inner"

        def detect(self, frame):
            return [Detection("cup", 0.8, (1, 1, 5, 5))]

    preview = Preview(ZonePresence((0, 0, 1, 1)), port=0, host="127.0.0.1")
    try:
        detector = Previewed(Inner(), preview)
        assert detector.name == "inner"
        assert [d.label for d in detector.detect(np.zeros((20, 20, 3), np.uint8))] == ["cup"]
    finally:
        preview.close()
