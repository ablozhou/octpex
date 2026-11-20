"""
Weather Arm HTTP & REST Worker Server
Exposes the Octo-Arm endpoints driven by arm.yaml:
1. GET  /manifest         - Exposes this arm's ArmManifest for Octo-Brain registration
2. POST /api/v1/dispatch  - Receives NervousEnvelope dispatch from Octo-Brain
3. GET  /weather          - Direct endpoint (e.g. /weather?location=Beijing&unit=c)

Auto-Registration:
Automatically registers with Octpex Brain (default http://127.0.0.1:8080/api/v1/register)
and maintains periodic heartbeats every 5 seconds.
"""

import os
import sys
import json
import time
import threading
import urllib.parse
import urllib.request
import urllib.error
from http.server import HTTPServer, BaseHTTPRequestHandler

# Ensure both bridge/python and current folder are in sys.path
sys.path.insert(0, os.path.abspath(os.path.join(os.path.dirname(__file__), "..", "..", "bridge", "python")))
sys.path.insert(0, os.path.abspath(os.path.dirname(__file__)))

from octpex import (
    NervousEnvelope,
    InMemorySubstrateClient,
)
from main import WeatherArmWorker

# Initialize Arm Worker from local arm.yaml
substrate = InMemorySubstrateClient()
worker = WeatherArmWorker.from_manifest(substrate=substrate)
manifest = worker.get_registration_manifest()


def start_auto_registration(brain_url: str, arm_port: int, arm_manifest: dict):
    """
    Background daemon thread that auto-registers this arm to Octpex Central Brain
    and sends periodic heartbeats every 5 seconds.
    """
    def _worker():
        register_url = f"{brain_url.rstrip('/')}/api/v1/register"
        heartbeat_url = f"{brain_url.rstrip('/')}/api/v1/heartbeat"
        arm_id = arm_manifest.get("arm_id", "arm-weather-mcp")
        
        # Clone manifest and set dynamic endpoint
        reg_payload = dict(arm_manifest)
        reg_payload["endpoint"] = f"http://127.0.0.1:{arm_port}"

        # 1. Registration loop (retries if Octpex central brain is starting up)
        registered = False
        while not registered:
            try:
                data = json.dumps(reg_payload).encode("utf-8")
                req = urllib.request.Request(
                    register_url,
                    data=data,
                    headers={"Content-Type": "application/json"},
                    method="POST",
                )
                with urllib.request.urlopen(req, timeout=3) as resp:
                    if resp.status in (200, 201):
                        print(f"[Auto-Register] Registered arm '{arm_id}' successfully with Octpex Brain at {brain_url}")
                        registered = True
            except Exception:
                time.sleep(2)

        # 2. Periodic heartbeat loop
        while True:
            try:
                hb_data = json.dumps({
                    "arm_id": arm_id,
                    "status": "HEALTHY",
                    "queue_depth": 0,
                }).encode("utf-8")
                req = urllib.request.Request(
                    heartbeat_url,
                    data=hb_data,
                    headers={"Content-Type": "application/json"},
                    method="POST",
                )
                with urllib.request.urlopen(req, timeout=3) as resp:
                    pass
            except Exception:
                pass
            time.sleep(5)

    t = threading.Thread(target=_worker, daemon=True, name="octpex-arm-registrar")
    t.start()


class WeatherHTTPRequestHandler(BaseHTTPRequestHandler):
    def _send_json(self, status_code: int, data: dict):
        response_bytes = json.dumps(data, ensure_ascii=False, indent=2).encode("utf-8")
        self.send_response(status_code)
        self.send_header("Content-Type", "application/json; charset=utf-8")
        self.send_header("Content-Length", str(len(response_bytes)))
        self.send_header("Access-Control-Allow-Origin", "*")
        self.end_headers()
        self.wfile.write(response_bytes)

    def do_OPTIONS(self):
        self.send_response(204)
        self.send_header("Access-Control-Allow-Origin", "*")
        self.send_header("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
        self.send_header("Access-Control-Allow-Headers", "Content-Type")
        self.end_headers()

    def do_GET(self):
        parsed = urllib.parse.urlparse(self.path)
        if parsed.path in ["/", "/health"]:
            self._send_json(200, {
                "status": "HEALTHY",
                "arm_id": manifest["arm_id"],
                "role_name": manifest["role_name"],
                "capabilities": manifest["capability_tags"],
                "slm_profile": manifest["slm_profile"],
                "endpoints": {
                    "GET  /manifest": "ArmManifest registration card for Octo-Brain",
                    "POST /api/v1/dispatch": "Biomimetic envelope dispatch",
                    "GET  /weather?location=Beijing&unit=c": "Direct query",
                }
            })
            return

        # Expose arm registration manifest to Octo-Brain
        if parsed.path == "/manifest":
            self._send_json(200, manifest)
            return

        if parsed.path == "/weather":
            qs = urllib.parse.parse_qs(parsed.query)
            location = qs.get("location", ["Beijing"])[0]
            unit = qs.get("unit", ["c"])[0]

            envelope = NervousEnvelope(
                envelope_id=f"env-http-get-{int(time.time() * 1000)}",
                trace_id="trace-http-get",
                session_id="http-session",
                plane="VERTICAL",
                message_type="TASK_DISPATCH",
                source_node="http-client",
                target_capability="get_current_temperature",
                parameters={"location": location, "unit": unit},
            )
            try:
                result = worker.execute_and_persist(envelope)
                self._send_json(200, {
                    "status": result.status,
                    "data": json.loads(result.output_message),
                    "substrate_ref": result.substrate_refs[0].uri if result.substrate_refs else None,
                })
            except Exception as e:
                self._send_json(500, {"error": f"Weather query execution failed: {e}"})
            return

        self._send_json(404, {"error": f"Endpoint {parsed.path} not found"})

    def do_POST(self):
        parsed = urllib.parse.urlparse(self.path)
        content_length = int(self.headers.get("Content-Length", 0))
        body = self.rfile.read(content_length) if content_length > 0 else b"{}"

        try:
            req_data = json.loads(body.decode("utf-8")) if body else {}
        except Exception as e:
            self._send_json(400, {"error": f"Invalid JSON body: {e}"})
            return

        # Biomimetic envelope dispatch from Octo-Brain
        if parsed.path == "/api/v1/dispatch":
            target_cap = req_data.get("target_capability", "get_current_temperature")
            params = req_data.get("parameters", {})
            env_id = req_data.get("envelope_id", f"env-curl-{int(time.time() * 1000)}")
            session_id = req_data.get("session_id", "curl-session")

            envelope = NervousEnvelope(
                envelope_id=env_id,
                trace_id=req_data.get("trace_id", "trace-curl"),
                session_id=session_id,
                plane="VERTICAL",
                message_type="TASK_DISPATCH",
                source_node=req_data.get("source_node", "octo-brain"),
                target_capability=target_cap,
                parameters=params,
            )
            try:
                result = worker.execute_and_persist(envelope)
                self._send_json(200, {
                    "arm_id": worker.config.arm_id,
                    "step_result": result.to_dict(),
                })
            except Exception as e:
                import traceback
                traceback.print_exc()
                self._send_json(500, {
                    "arm_id": worker.config.arm_id,
                    "error": str(e),
                    "step_result": {
                        "status": "FAILED",
                        "output_message": f"Arm execution error: {e}",
                        "substrate_refs": [],
                    }
                })
            return

        self._send_json(404, {"error": f"Endpoint {parsed.path} not found"})


def run_server(port: int = 8081, brain_url: str = "http://127.0.0.1:8080"):
    manifest["endpoint"] = f"http://127.0.0.1:{port}"
    server_address = ("", port)
    httpd = HTTPServer(server_address, WeatherHTTPRequestHandler)
    print("=" * 80)
    print(f"Octpex Weather Arm Worker listening on http://127.0.0.1:{port}")
    print(f"  • Arm ID        : {manifest['arm_id']}")
    print(f"  • Capabilities  : {manifest['capability_tags']}")
    print(f"  • Central Brain : {brain_url}")
    print(f"  • Config Source : arm.yaml (SLM: {manifest['slm_profile']['model']})")
    print("Endpoints:")
    print(f"  • GET  http://127.0.0.1:{port}/manifest         (ArmManifest for Octo-Brain)")
    print(f"  • POST http://127.0.0.1:{port}/api/v1/dispatch (Envelope dispatch)")
    print(f"  • GET  http://127.0.0.1:{port}/weather?location=Beijing&unit=c")
    print("=" * 80)

    # Launch background auto-registration and heartbeat daemon
    start_auto_registration(brain_url, port, manifest)

    try:
        httpd.serve_forever()
    except KeyboardInterrupt:
        print("\nStopping Weather Arm Worker...")
        httpd.server_close()


if __name__ == "__main__":
    port_arg = int(sys.argv[1]) if len(sys.argv) > 1 and sys.argv[1].isdigit() else 8081
    brain_arg = os.environ.get("OCTPEX_BRAIN_URL", "http://127.0.0.1:8080")
    if len(sys.argv) > 2 and sys.argv[2].startswith("http"):
        brain_arg = sys.argv[2]
    run_server(port_arg, brain_arg)
