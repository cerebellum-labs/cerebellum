"""HTTP to the server. Every request starts here, on the device; the server never calls back."""

from __future__ import annotations

from typing import Any

import httpx


class Offline(Exception):
    """No network, a timeout or a 5xx: try again on the next round."""


class Unauthorized(Exception):
    """401: the token is wrong, was replaced, or its device was deleted."""


class Rejected(Exception):
    """The server won't take this request as it is (400, 413, other 4xx)."""

    def __init__(self, status: int, message: str) -> None:
        super().__init__(f"{status}: {message}")
        self.status = status
        self.message = message


class Client:
    def __init__(self, base_url: str, token: str, agent_version: str) -> None:
        self._http = httpx.Client(
            base_url=base_url.rstrip("/"),
            headers={
                "Authorization": f"Bearer {token}",
                "User-Agent": f"cerebellum-agent/{agent_version}",
            },
        )

    def post(self, path: str, body: dict[str, Any], timeout: float) -> dict[str, Any]:
        try:
            resp = self._http.post(path, json=body, timeout=timeout)
        except httpx.HTTPError as exc:  # transport errors and timeouts
            raise Offline(str(exc) or type(exc).__name__) from exc
        if resp.status_code == 200:
            try:
                return resp.json()
            except ValueError as exc:
                raise Offline("the server sent a body that isn't JSON") from exc
        if resp.status_code == 401:
            raise Unauthorized(_message(resp))
        if resp.status_code >= 500:
            raise Offline(f"{resp.status_code}: {_message(resp)}")
        raise Rejected(resp.status_code, _message(resp))

    def close(self) -> None:
        self._http.close()


def _message(resp: httpx.Response) -> str:
    try:
        error = resp.json()["error"]
        return error["message"] if isinstance(error, dict) else str(error)
    except (ValueError, KeyError, TypeError):
        return resp.text[:200] or resp.reason_phrase
