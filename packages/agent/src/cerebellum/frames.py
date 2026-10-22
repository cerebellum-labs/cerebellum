"""Frames are shrunk once, here, so the queue and the network carry 30-60 KB, not a camera's full size."""

from __future__ import annotations

import io

from PIL import Image

from .errors import RulesError

MAX_FRAME_BYTES = 1 << 20


def prepare(jpeg: bytes, size: tuple[int, int], quality: int) -> tuple[bytes, int, int]:
    """Return (jpeg, width, height) no larger than `size`. A JPEG already small enough passes untouched."""
    try:
        image: Image.Image = Image.open(io.BytesIO(jpeg))
        image.load()
    except Exception as exc:
        raise RulesError(f"frame is not a readable image: {exc}") from exc
    width, height = image.size
    if image.format == "JPEG" and width <= size[0] and height <= size[1] and len(jpeg) <= MAX_FRAME_BYTES:
        return jpeg, width, height
    image = image.convert("RGB")
    image.thumbnail(size)
    out = io.BytesIO()
    image.save(out, format="JPEG", quality=quality)
    return out.getvalue(), image.width, image.height
