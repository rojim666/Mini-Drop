import json
import os
import subprocess
import sys
import time
import urllib.error
import urllib.request


API_PORT = os.environ.get("MINIDROP_API_PORT", "8080")
API_BASE = os.environ.get("MINIDROP_API_BASE_URL", f"http://127.0.0.1:{API_PORT}").rstrip("/")
AGENT_ID = os.environ.get("MINIDROP_TARGET_AGENT_ID", "drop_agent")
TARGET_PID = int(os.environ.get("MINIDROP_TARGET_PID", "1"))
COMPOSE_FILE = os.environ.get("MINIDROP_COMPOSE_FILE", "deploy/docker-compose.yml")


def request_json(path: str, method: str = "GET", body: dict | None = None) -> dict:
    data = None
    headers = {}
    if body is not None:
        data = json.dumps(body).encode("utf-8")
        headers["Content-Type"] = "application/json"

    req = urllib.request.Request(f"{API_BASE}{path}", method=method, data=data, headers=headers)
    with urllib.request.urlopen(req, timeout=10) as response:
        return json.loads(response.read().decode("utf-8"))


def wait_for_api(deadline: float) -> None:
    while time.time() < deadline:
        try:
            request_json("/healthz")
            return
        except Exception:
            time.sleep(1)
    raise RuntimeError(f"API did not become ready at {API_BASE}/healthz")


def wait_for_agent_status(deadline: float, status: str) -> None:
    while time.time() < deadline:
        agents = request_json("/api/v1/agents").get("agents", [])
        for agent in agents:
            if agent.get("id") == AGENT_ID and agent.get("status") == status:
                print(f"agent {AGENT_ID} status={status}")
                return
        time.sleep(1)
    raise RuntimeError(f"agent {AGENT_ID} did not become {status}")


def stop_agent_container() -> None:
    subprocess.run(
        ["docker", "compose", "-f", COMPOSE_FILE, "stop", "agent"],
        check=True,
    )


def create_task() -> str:
    body = {
        "target_pid": TARGET_PID,
        "target_agent_id": AGENT_ID,
        "sample_duration_sec": 60,
        "sample_rate_hz": 99,
        "collector_type": "mock-perf",
    }
    created = request_json("/api/v1/tasks", method="POST", body=body)
    task_id = created["task"]["id"]
    print(f"created task {task_id}")
    return task_id


def wait_for_failed_task(task_id: str, deadline: float) -> None:
    while time.time() < deadline:
        task = request_json(f"/api/v1/tasks/{task_id}")["task"]
        print(f"task {task_id} status={task['status']} reason={task['status_reason']}")
        if task["status"] == "FAILED" and task["status_reason"] == "target agent offline":
            return
        if task["status"] in {"DONE", "FAILED"}:
            raise RuntimeError(f"expected target agent offline failure, got {task['status']} / {task['status_reason']}")
        time.sleep(2)
    raise RuntimeError(f"task {task_id} did not fail after agent offline")


def assert_offline_audit_log() -> None:
    logs = request_json("/api/v1/audit-logs").get("audit_logs", [])
    for item in logs:
        if (
            item.get("entity_type") == "agent"
            and item.get("entity_id") == AGENT_ID
            and item.get("action") == "offline"
            and item.get("reason") == "agent heartbeat timed out"
        ):
            print(f"audit offline log found for {AGENT_ID}")
            return
    raise RuntimeError(f"offline audit log not found for {AGENT_ID}")


def main() -> int:
    deadline = time.time() + 120
    wait_for_api(deadline)
    wait_for_agent_status(deadline, "ONLINE")
    task_id = create_task()
    stop_agent_container()
    wait_for_agent_status(deadline, "OFFLINE")
    wait_for_failed_task(task_id, deadline)
    assert_offline_audit_log()
    return 0


if __name__ == "__main__":
    try:
        raise SystemExit(main())
    except (RuntimeError, urllib.error.URLError, urllib.error.HTTPError, subprocess.CalledProcessError) as exc:
        print(str(exc), file=sys.stderr)
        raise SystemExit(1)
