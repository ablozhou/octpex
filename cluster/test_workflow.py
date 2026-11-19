"""Verify two kernel containers in a new, isolated Compose project."""
import json
import pathlib
import sys
import subprocess
import time
import urllib.request
import uuid


def request(port, path, data=None):
    body = None if data is None else json.dumps(data).encode()
    req = urllib.request.Request(f"http://127.0.0.1:{port}/api/v1/{path}", body,
                                 {"Content-Type": "application/json"})
    with urllib.request.urlopen(req, timeout=12) as response:
        return json.load(response)


def wait_for(condition):
    deadline = time.monotonic() + 25
    while time.monotonic() < deadline:
        try:
            if condition():
                return
        except OSError:
            pass
        time.sleep(0.25)
    raise RuntimeError("cluster did not reach the expected state within 25 seconds")


def verify():
    root = pathlib.Path(__file__).resolve().parents[1]
    project = "octpex-check-" + uuid.uuid4().hex[:12]
    compose = ["docker", "compose", "-p", project, "-f", str(root / "deploy/compose.acceptance.yaml")]

    def mapped_port(service):
        address = subprocess.check_output(compose + ["port", service, "8080"], text=True).strip()
        return int(address.rsplit(":", 1)[1])

    # Only the project created by this invocation is removed.
    subprocess.run(["docker", "info", "--format", "{{.ServerVersion}}"], check=True, timeout=20)
    try:
        subprocess.run(compose + ["build", "entry"], check=True)
        subprocess.run(compose + ["up", "-d", "--no-build", "--wait", "--wait-timeout", "120"], check=True)
        a, b = mapped_port("entry"), mapped_port("worker")
        wait_for(lambda: request(a, "topology")["arm_count"] == 1)
        trace = uuid.uuid4().hex
        body = {"target_capability": "distill-telemetry", "envelope_id": trace,
                "session_id": trace, "trace_id": trace, "parameters": {"log": "\n".join(["WARN x", "WARN x"])}}
        result = request(a, "dispatch", body)
        assert result["status"] == "SUCCESS" and result["metadata"]["trace_id"] == trace
        arm = request(b, "topology")["active_arms"][0]["arm_id"]
        assert result["metadata"]["worker_id"] == arm
        ref = result["substrate_refs"][0]
        assert ref["version"] == 1 and request(a, "dispatch", body)["substrate_refs"] == [ref]
        stored = subprocess.check_output(compose + [
            "exec", "-T", "redis", "redis-cli", "--raw", "HMGET",
            "octpex:sub:" + ref["uri"], "data", "version"], text=True)
        assert stored.splitlines() == ["WARN x", "1"]
        entry_tasks = request(a, "finops")["total_tasks"]
        worker_tasks = request(b, "finops")["total_tasks"]
        assert entry_tasks == 0 and worker_tasks == 1, (entry_tasks, worker_tasks)
        print(json.dumps({"trace_id": trace, "worker_id": arm, "redis_uri": ref["uri"],
                          "redis_data": stored.splitlines()[0], "redis_version": ref["version"],
                          "entry_tasks": entry_tasks, "worker_tasks_after_replay": worker_tasks}), flush=True)
        # Disable restart only on this temporary worker so its lease can expire.
        worker = subprocess.check_output(compose + ["ps", "-q", "worker"], text=True).strip()
        subprocess.run(["docker", "update", "--restart=no", worker], check=True)
        started = time.monotonic()
        subprocess.run(compose + ["kill", "-s", "SIGKILL", "worker"], check=True)
        wait_for(lambda: request(a, "topology")["arm_count"] == 0)
        print(json.dumps({"worker_id": arm, "remaining_workers": 0,
                          "observed_removal_seconds": round(time.monotonic() - started, 3)}), flush=True)
    except Exception:
        subprocess.run(compose + ["logs", "--tail=40"], stdout=sys.stderr)
        raise
    finally:
        subprocess.run(compose + ["down", "--volumes", "--remove-orphans"], check=True)
        tag = "octpex-acceptance:" + project
        if subprocess.run(["docker", "image", "inspect", tag], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL).returncode == 0:
            subprocess.run(["docker", "image", "rm", tag], check=True)


if __name__ == "__main__":
    verify()
    print("Two-node dispatch, CAS replay, Redis readback and worker lease expiry passed.")
