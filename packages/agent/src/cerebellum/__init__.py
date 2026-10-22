"""Cerebellum agent: the on-device half of the sync layer between a robot's local and cloud models."""

from importlib.metadata import PackageNotFoundError, version

from .agent import Cerebellum
from .errors import RulesError
from .types import Correction, Decision

try:
    __version__ = version("cerebellum-agent")
except PackageNotFoundError:  # running from a source tree that was never installed
    __version__ = "0+unknown"

__all__ = ["Cerebellum", "Correction", "Decision", "RulesError", "__version__"]
