import json
import sys
import time
import urllib.error
import urllib.request


API_BASE = "http://127.0.0.1:8080"


def request_json(path: str, method: str = "GET", body: dict | None = None) -> dict:
    data = None
    headers = {}
    if body is not None:
        data = json.dumps(body).encode("utf-8")
        headers["Content-Type"] = "application/json"

    req = urllib.request.Request(f"{API_BASE}{path}", method=method, data=data, headers=headers)
    with urllib.request.urlopen(req, timeout=10) as response:
        return json.loads(response.read().decode("utf-8"))


def main() -> int:
    if len(sys.argv) < 2:
        print("usage: python scripts/demo/smoke_e2e.py <pid> [agent_id]", file=sys.stderr)
        return 1

    target_pid = int(sys.argv[1])
    target_agent_id = sys.argv[2] if len(sys.argv) > 2 else ""

    body = {
        "target_pid": target_pid,
        "target_agent_id": target_agent_id,
        "sample_duration_sec": 15,
        "sample_rate_hz": 99,
        "collector_type": "mock-perf",
    }

    created = request_json("/api/v1/tasks", method="POST", body=body)
    task_id = created["task"]["id"]
    print(f"created task {task_id}")

    deadline = time.time() + 30
    while time.time() < deadline:
        task = request_json(f"/api/v1/tasks/{task_id}")["task"]
        print(f"task {task_id} status={task['status']} reason={task['status_reason']}")
        if task["status"] in {"DONE", "FAILED"}:
            if task["status"] == "DONE":
                result = task.get("result") or {}
                print(f"flamegraph={result.get('flamegraph_url')}")
                print(f"topn={result.get('topn_url')}")
                return 0
            return 2
        time.sleep(2)

    print("timed out waiting for task completion", file=sys.stderr)
    return 3


if __name__ == "__main__":
    try:
        raise SystemExit(main())
    except urllib.error.HTTPError as exc:
        print(exc.read().decode("utf-8"), file=sys.stderr)
        raise
