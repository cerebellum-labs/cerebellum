"""A stand-in for the Cerebellum server that follows packages/server/AGENT_API.md closely enough to
exercise the agent: it answers /decide, stores /sync, reviews uncertain decisions and can go offline."""

from __future__ import annotations

import json
import threading
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from typing import Any

from cerebellum.clock import now_iso


class FakeServer:
    def __init__(self, token: str = "test-token") -> None:
        self.token = token
        self.online = True
        self.decide_action = "basket"
        self.review_action: str | None = None
        self.fail_decide = 0  # how many answers come back as retryable errors
        self.reject_decide = False  # answer every item with retry:false
        self.max_items = 99  # a request with more items gets a 413
        self.reject_sync = False  # a 400 for every /sync that carries items
        self.decisions: dict[str, dict[str, Any]] = {}
        self.frames: dict[str, int] = {}
        self.frame_uploads: dict[str, int] = {}
        self.requests: list[tuple[str, dict[str, Any]]] = []
        self.agent_version: str | None = None
        self._lock = threading.Lock()
        outer = self

        class Handler(BaseHTTPRequestHandler):
            def log_message(self, *args: Any) -> None:
                pass

            def do_POST(self) -> None:
                body = json.loads(self.rfile.read(int(self.headers["Content-Length"])) or b"{}")
                status, payload = outer._handle(self.path, self.headers.get("Authorization", ""), body)
                data = json.dumps(payload).encode()
                self.send_response(status)
                self.send_header("Content-Type", "application/json")
                self.send_header("Content-Length", str(len(data)))
                self.end_headers()
                self.wfile.write(data)

        self._httpd = ThreadingHTTPServer(("127.0.0.1", 0), Handler)
        self.url = f"http://127.0.0.1:{self._httpd.server_port}"
        self._thread = threading.Thread(target=self._httpd.serve_forever, daemon=True)
        self._thread.start()

    def close(self) -> None:
        self._httpd.shutdown()
        self._httpd.server_close()

    def count(self, path: str) -> int:
        return sum(1 for p, _ in self.requests if p == path)

    # --- the API -------------------------------------------------------------

    def _handle(self, path: str, auth: str, body: dict[str, Any]) -> tuple[int, dict[str, Any]]:
        with self._lock:
            self.requests.append((path, body))
            if not self.online:
                return 503, _error("internal_error", "offline")
            if auth != f"Bearer {self.token}":
                return 401, _error("unauthorized", "the device token is wrong")
            if len(body.get("items", [])) > self.max_items:
                return 413, _error("payload_too_large", "too many items")
            if path == "/api/v1/sync" and self.reject_sync and body.get("items"):
                return 400, _error("bad_request", "nope")
            if path == "/api/v1/decide":
                return 200, self._decide(body)
            if path == "/api/v1/sync":
                return 200, self._sync(body)
            return 404, _error("not_found", "no such path")

    def _save(self, item: dict[str, Any]) -> dict[str, Any]:
        sent = item["decision"]
        old = self.decisions.get(sent["id"])
        if old is None or sent["revision"] > old["revision"]:
            keep = {k: old[k] for k in ("received_at",) if old} if old else {}
            new = {**(old or {}), **sent, **keep}
            new.setdefault("received_at", now_iso())
            if old and old["action_class"] == "irreversible":
                new["action"] = old["action"]  # the server's
            self.decisions[sent["id"]] = new
        if item.get("frame"):
            self.frame_uploads[sent["id"]] = self.frame_uploads.get(sent["id"], 0) + 1
            self.frames.setdefault(sent["id"], len(item["frame"]))
        return self.decisions[sent["id"]]

    def _decide(self, body: dict[str, Any]) -> dict[str, Any]:
        decisions, errors = [], []
        for item in body["items"]:
            d = self._save(item)
            if self.reject_decide:
                errors.append({"decision_id": d["id"], "error": "options are not valid", "retry": False})
            elif self.fail_decide > 0:
                self.fail_decide -= 1
                errors.append({"decision_id": d["id"], "error": "the cloud model gave no answer", "retry": True})
            else:
                if d.get("action") is None:
                    d.update(
                        action=self.decide_action, source="server", model="fake-model", confidence=0.97,
                        reason="because", latency_ms=5, cost_usd=0.0004, answered_at=now_iso(),
                    )  # fmt: skip
                decisions.append(dict(d))
        return {"decisions": decisions, "errors": errors}

    def _sync(self, body: dict[str, Any]) -> dict[str, Any]:
        self.agent_version = body.get("agent_version")
        out = []
        for item in body["items"]:
            d = self._save(item)
            if d["needs_review"] and self.review_action and not d.get("reviewed_at"):
                d["reviewed_at"] = now_iso()
                if self.review_action != d["action"]:
                    d["corrected_action"] = self.review_action
            out.append(dict(d))
        corrections = [
            {
                "id": d["id"], "decision_id": d["id"], "action": d["corrected_action"], "reason": "it is a ball",
                "model": "fake-model", "created_at": d["reviewed_at"], "applied_at": None,
            }
            for d in self.decisions.values()
            if d.get("corrected_action") and not d.get("correction_applied_at")
        ]  # fmt: skip
        return {"decisions": out, "corrections": corrections, "errors": [], "server_time": now_iso()}


def _error(code: str, message: str) -> dict[str, Any]:
    return {"error": {"code": code, "message": message}}
