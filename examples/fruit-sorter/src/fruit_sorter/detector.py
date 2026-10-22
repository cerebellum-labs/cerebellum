"""Finding things in a frame: a COCO detector, the part of the example that runs on the Pi."""

from __future__ import annotations

from dataclasses import dataclass
from pathlib import Path
from typing import Protocol

import cv2
import numpy as np

COCO_LABELS = [
    "person",
    "bicycle",
    "car",
    "motorcycle",
    "airplane",
    "bus",
    "train",
    "truck",
    "boat",
    "traffic_light",
    "fire_hydrant",
    "stop_sign",
    "parking_meter",
    "bench",
    "bird",
    "cat",
    "dog",
    "horse",
    "sheep",
    "cow",
    "elephant",
    "bear",
    "zebra",
    "giraffe",
    "backpack",
    "umbrella",
    "handbag",
    "tie",
    "suitcase",
    "frisbee",
    "skis",
    "snowboard",
    "sports_ball",
    "kite",
    "baseball_bat",
    "baseball_glove",
    "skateboard",
    "surfboard",
    "tennis_racket",
    "bottle",
    "wine_glass",
    "cup",
    "fork",
    "knife",
    "spoon",
    "bowl",
    "banana",
    "apple",
    "sandwich",
    "orange",
    "broccoli",
    "carrot",
    "hot_dog",
    "pizza",
    "donut",
    "cake",
    "chair",
    "couch",
    "potted_plant",
    "bed",
    "dining_table",
    "toilet",
    "tv",
    "laptop",
    "mouse",
    "remote",
    "keyboard",
    "cell_phone",
    "microwave",
    "oven",
    "toaster",
    "sink",
    "refrigerator",
    "book",
    "clock",
    "vase",
    "scissors",
    "teddy_bear",
    "hair_drier",
    "toothbrush",
]
assert len(COCO_LABELS) == 80

MODEL_URL = "https://github.com/Megvii-BaseDetection/YOLOX/releases/download/0.1.1rc0/yolox_nano.onnx"


@dataclass(frozen=True)
class Detection:
    label: str
    confidence: float
    box: tuple[int, int, int, int]  # x0, y0, x1, y1 in the frame

    @property
    def center(self) -> tuple[float, float]:
        return (self.box[0] + self.box[2]) / 2, (self.box[1] + self.box[3]) / 2


class Detector(Protocol):
    name: str

    def detect(self, frame: np.ndarray) -> list[Detection]: ...


class YoloxDetector:
    """YOLOX (Apache-2.0), trained on COCO, through ONNX Runtime on the CPU."""

    name = "yolox-nano"

    def __init__(self, model_path: str | Path, score_threshold: float = 0.15, nms_threshold: float = 0.45) -> None:
        import onnxruntime as ort

        self._session = ort.InferenceSession(str(model_path), providers=["CPUExecutionProvider"])
        inp = self._session.get_inputs()[0]
        self._input_name = inp.name
        self._size = (int(inp.shape[2]), int(inp.shape[3]))  # height, width
        self._score_threshold = score_threshold
        self._nms_threshold = nms_threshold
        self._grid, self._strides = _grids(self._size)

    def detect(self, frame: np.ndarray) -> list[Detection]:
        blob, ratio = _letterbox(frame, self._size)
        raw = self._session.run(None, {self._input_name: blob[None]})[0][0]
        return self._decode(raw, ratio, frame.shape[1], frame.shape[0])

    def _decode(self, raw: np.ndarray, ratio: float, width: int, height: int) -> list[Detection]:
        out = raw.copy()
        out[:, :2] = (out[:, :2] + self._grid) * self._strides
        out[:, 2:4] = np.exp(out[:, 2:4]) * self._strides
        scores = out[:, 4:5] * out[:, 5:]
        class_ids = scores.argmax(1)
        confidences = scores[np.arange(len(scores)), class_ids]
        keep = confidences >= self._score_threshold
        if not keep.any():
            return []
        out, class_ids, confidences = out[keep], class_ids[keep], confidences[keep]
        cx, cy, w, h = out[:, 0], out[:, 1], out[:, 2], out[:, 3]
        boxes = np.stack([cx - w / 2, cy - h / 2, w, h], axis=1) / ratio  # x, y, w, h in the frame

        found: list[Detection] = []
        for class_id in np.unique(class_ids):
            idx = np.flatnonzero(class_ids == class_id)
            kept = cv2.dnn.NMSBoxes(
                boxes[idx].tolist(), confidences[idx].tolist(), self._score_threshold, self._nms_threshold
            )
            for k in np.array(kept).flatten():
                x, y, bw, bh = boxes[idx[k]]
                x0, y0 = max(0, int(x)), max(0, int(y))
                x1, y1 = min(width, int(x + bw)), min(height, int(y + bh))
                if x1 > x0 and y1 > y0:
                    found.append(Detection(COCO_LABELS[int(class_id)], float(confidences[idx[k]]), (x0, y0, x1, y1)))
        return sorted(found, key=lambda d: -d.confidence)


def _letterbox(frame: np.ndarray, size: tuple[int, int]) -> tuple[np.ndarray, float]:
    """Fit the frame in the model's square without distorting it, as YOLOX was trained: BGR, 0-255, grey padding."""
    height, width = frame.shape[:2]
    ratio = min(size[0] / height, size[1] / width)
    resized = cv2.resize(frame, (int(width * ratio), int(height * ratio)), interpolation=cv2.INTER_LINEAR)
    padded = np.full((size[0], size[1], 3), 114, dtype=np.uint8)
    padded[: resized.shape[0], : resized.shape[1]] = resized
    return np.ascontiguousarray(padded.transpose(2, 0, 1), dtype=np.float32), ratio


def _grids(size: tuple[int, int]) -> tuple[np.ndarray, np.ndarray]:
    """Where each of the model's outputs sits in the input, and how coarse its cell is."""
    grids, strides = [], []
    for stride in (8, 16, 32):
        hsize, wsize = size[0] // stride, size[1] // stride
        xv, yv = np.meshgrid(np.arange(wsize), np.arange(hsize))
        grids.append(np.stack((xv, yv), 2).reshape(-1, 2))
        strides.append(np.full((hsize * wsize, 1), stride))
    return np.concatenate(grids, 0).astype(np.float32), np.concatenate(strides, 0).astype(np.float32)
