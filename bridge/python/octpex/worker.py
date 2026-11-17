import json
import logging
import time
import urllib.request
import urllib.error
from typing import Optional, Dict, Any, Callable
from .arm import OctoArmWorker
from .protocol import NervousEnvelope, StepResult

logger = logging.getLogger("octpex.worker")


class OctoArmDaemon:
    """
    OctoArmDaemon is the standalone execution runner for a Python Octo-Arm worker.
    It can execute tasks locally or communicate with the Octpex Go kernel via REST.
    """

    def __init__(
        self,
        worker: OctoArmWorker,
        server_url: str = "http://localhost:8080",
        poll_interval: float = 1.0,
    ):
        self.worker = worker
        self.server_url = server_url.rstrip("/")
        self.poll_interval = poll_interval
        self._running = False

    def handle_envelope(self, envelope: NervousEnvelope) -> StepResult:
        """
        Executes a single envelope through the worker's biomimetic reflex loop.
        """
        logger.info(
            f"Daemon [{self.worker.config.arm_id}] processing envelope {envelope.envelope_id}"
        )
        return self.worker.execute_envelope(envelope)

    def dispatch_to_kernel(self, envelope: NervousEnvelope) -> Dict[str, Any]:
        """
        Sends a NervousEnvelope to the Octpex kernel's HTTP dispatch endpoint.
        """
        url = f"{self.server_url}/api/v1/dispatch"
        payload = json.dumps(envelope.to_dict()).encode("utf-8")
        req = urllib.request.Request(
            url,
            data=payload,
            headers={"Content-Type": "application/json"},
            method="POST",
        )

        with urllib.request.urlopen(req, timeout=5.0) as resp:
            data = resp.read()
            return json.loads(data.decode("utf-8"))

    def get_kernel_topology(self) -> Dict[str, Any]:
        """
        Queries the current active topology from the Octpex kernel.
        """
        url = f"{self.server_url}/api/v1/topology"
        req = urllib.request.Request(url, method="GET")
        with urllib.request.urlopen(req, timeout=5.0) as resp:
            return json.loads(resp.read().decode("utf-8"))

    def get_kernel_finops(self) -> Dict[str, Any]:
        """
        Queries FinOps ROI metrics from the Octpex kernel.
        """
        url = f"{self.server_url}/api/v1/finops"
        req = urllib.request.Request(url, method="GET")
        with urllib.request.urlopen(req, timeout=5.0) as resp:
            return json.loads(resp.read().decode("utf-8"))

    def stop(self) -> None:
        self._running = False
