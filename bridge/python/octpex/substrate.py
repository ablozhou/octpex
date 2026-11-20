import json
from abc import ABC, abstractmethod
from typing import Tuple, Dict, Any, Optional


class SubstrateClient(ABC):
    """Abstract client contract for interacting with Octpex Shared Data Substrate."""

    @abstractmethod
    def get_artifact(self, uri: str) -> Tuple[bytes, int]:
        """Fetches payload and version for a given sub:// URI."""
        pass

    @abstractmethod
    def put_artifact(self, uri: str, expected_version: int, data: bytes, ttl_seconds: int = 3600) -> int:
        """Puts payload with atomic CAS check, returning new monotonic version number."""
        pass


class InMemorySubstrateClient(SubstrateClient):
    """In-memory substrate client for testing and zero-dependency local pipelines."""

    def __init__(self):
        self._store: Dict[str, Dict[str, Any]] = {}

    def get_artifact(self, uri: str) -> Tuple[bytes, int]:
        if uri not in self._store:
            raise KeyError(f"Artifact not found: {uri}")
        item = self._store[uri]
        return item["data"], item["version"]

    def put_artifact(self, uri: str, expected_version: int, data: bytes, ttl_seconds: int = 3600) -> int:
        if uri in self._store:
            curr_ver = self._store[uri]["version"]
            if expected_version == -1:
                next_ver = curr_ver + 1
                self._store[uri] = {"data": data, "version": next_ver}
                return next_ver
            if expected_version == 0:
                raise ValueError(f"CAS conflict: artifact already exists: {uri}")
            if curr_ver != expected_version:
                raise ValueError(f"CAS conflict: version mismatch {curr_ver} != {expected_version}")
            next_ver = curr_ver + 1
            self._store[uri] = {"data": data, "version": next_ver}
            return next_ver
        else:
            if expected_version != 0 and expected_version != -1:
                raise ValueError(f"CAS conflict: artifact does not exist: {uri}")
            self._store[uri] = {"data": data, "version": 1}
            return 1
