"""What the robot sees, for aiming the camera: the newest frame with the zone and the detections drawn on it, served as
a stream any browser can open. Off unless asked for, and without a password: it shows the camera to the network."""

from __future__ import annotations

import threading
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

import cv2
import numpy as np

from .detector import Detection, Detector
from .presence import ZonePresence

_PAGE = (
    b"<!doctype html><title>fruit-sorter</title>"
    b"<body style='margin:0;background:#111'><img src='/stream' style='width:100%'>"
)
_ZONE = (0, 220, 255)  # BGR
_PERSON = (60, 60, 255)
_OTHER = (80, 220, 80)


class Preview:
    def __init__(self, presence: ZonePresence, port: int, host: str = "0.0.0.0") -> None:  # noqa: S104 - the point
        self._presence = presence
        self._lock = threading.Condition()
        self._jpeg: bytes | None = None
        self._version = 0
        self._server = ThreadingHTTPServer((host, port), self._handler())
        self._server.daemon_threads = True
        threading.Thread(target=self._server.serve_forever, name="preview", daemon=True).start()

    @property
    def port(self) -> int:
        return self._server.server_address[1]

    def show(self, frame: np.ndarray, detections: list[Detection]) -> None:
        image = frame.copy()
        x0, y0, x1, y1 = self._presence.rect(image.shape)
        cv2.rectangle(image, (x0, y0), (x1, y1), _ZONE, 1)
        for d in detections:
            color = _PERSON if d.label == "person" else _OTHER
            cv2.rectangle(image, d.box[:2], d.box[2:], color, 2)
            cv2.putText(image, f"{d.label} {d.confidence:.2f}", (d.box[0], max(12, d.box[1] - 5)),
                        cv2.FONT_HERSHEY_SIMPLEX, 0.5, color, 1)  # fmt: skip
        ok, jpeg = cv2.imencode(".jpg", image, [cv2.IMWRITE_JPEG_QUALITY, 70])
        if ok:
            with self._lock:
                self._jpeg, self._version = jpeg.tobytes(), self._version + 1
                self._lock.notify_all()

    def close(self) -> None:
        self._server.shutdown()
        self._server.server_close()

    def _next(self, seen: int) -> tuple[bytes, int] | None:
        """The frame after version `seen`, waiting a moment for it."""
        with self._lock:
            self._lock.wait_for(lambda: self._version != seen, timeout=2.0)
            return (self._jpeg, self._version) if self._jpeg and self._version != seen else None

    def _handler(self) -> type[BaseHTTPRequestHandler]:
        preview = self

        class Handler(BaseHTTPRequestHandler):
            def do_GET(self) -> None:  # noqa: N802
                if self.path == "/stream":
                    self._stream()
                elif self.path == "/snapshot.jpg":
                    got = preview._next(-1)
                    self._send(200, "image/jpeg", got[0]) if got else self._send(503, "text/plain", b"no frame yet")
                else:
                    self._send(200, "text/html", _PAGE)

            def _send(self, status: int, kind: str, body: bytes) -> None:
                self.send_response(status)
                self.send_header("Content-Type", kind)
                self.send_header("Content-Length", str(len(body)))
                self.send_header("Cache-Control", "no-store")
                self.end_headers()
                self.wfile.write(body)

            def _stream(self) -> None:
                self.send_response(200)
                self.send_header("Content-Type", "multipart/x-mixed-replace; boundary=frame")
                self.send_header("Cache-Control", "no-store")
                self.end_headers()
                seen = -1
                try:
                    while True:
                        got = preview._next(seen)
                        if got is None:
                            continue
                        jpeg, seen = got
                        self.wfile.write(
                            b"--frame\r\nContent-Type: image/jpeg\r\nContent-Length: %d\r\n\r\n" % len(jpeg)
                        )
                        self.wfile.write(jpeg + b"\r\n")
                except (BrokenPipeError, ConnectionResetError):
                    return

            def log_message(self, *args: object) -> None:
                pass

        return Handler


class Previewed:
    """A detector that also shows what it found."""

    def __init__(self, inner: Detector, preview: Preview) -> None:
        self.name = inner.name
        self._inner, self._preview = inner, preview

    def detect(self, frame: np.ndarray) -> list[Detection]:
        detections = self._inner.detect(frame)
        self._preview.show(frame, detections)
        return detections
