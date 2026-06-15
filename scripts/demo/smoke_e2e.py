import json
import os
import sys
import time
import urllib.error
import urllib.request
from argparse import ArgumentParser

from api_auth import auth_headers


API_BASE = os.environ.get("MINIDROP_API_BASE_URL", "http://127.0.0.1:8080").rstrip("/")


def parse_args() -> object:
    parser = ArgumentParser(description="Run a Mini-Drop API/Agent smoke task.")
    parser.add_argument("pid", type=int)
    parser.add_argument("agent_id", nargs="?", default="")
    parser.add_argument("collector_type", nargs="?", default="mock-perf")
    parser.add_argument("--expect-status", choices=["DONE", "FAILED"], default="DONE")
    parser.add_argument("--expect-reason-contains", default="")
    return parser.parse_args()


def request_json(path: str, method: str = "GET", body: dict | None = None) -> dict:
    data = None
    headers = auth_headers(API_BASE)
    if body is not None:
        data = json.dumps(body).encode("utf-8")
        headers["Content-Type"] = "application/json"

    req = urllib.request.Request(f"{API_BASE}{path}", method=method, data=data, headers=headers)
    with urllib.request.urlopen(req, timeout=10) as response:
        return json.loads(response.read().decode("utf-8"))


def main() -> int:
    args = parse_args()
    target_pid = args.pid
    target_agent_id = args.agent_id
    collector_type = args.collector_type
    sample_duration = 15 if collector_type == "perf" else 5
    sample_rate = 1 if collector_type == "ebpf-syscall" else 99
    deadline_seconds = sample_duration + 45

    body = {
        "target_pid": target_pid,
        "target_agent_id": target_agent_id,
        "sample_duration_sec": sample_duration,
        "sample_rate_hz": sample_rate,
        "collector_type": collector_type,
    }

    created = request_json("/api/v1/tasks", method="POST", body=body)
    task_id = created["task"]["id"]
    print(f"created task {task_id}")

    deadline = time.time() + deadline_seconds
    while time.time() < deadline:
        task = request_json(f"/api/v1/tasks/{task_id}")["task"]
        print(f"task {task_id} status={task['status']} reason={task['status_reason']}")
        if task["status"] in {"DONE", "FAILED"}:
            if task["status"] != args.expect_status:
                print(f"expected status {args.expect_status}, got {task['status']}", file=sys.stderr)
                return 4
            if args.expect_reason_contains and args.expect_reason_contains not in task["status_reason"]:
                print(
                    f"expected reason to contain {args.expect_reason_contains!r}, got {task['status_reason']!r}",
                    file=sys.stderr,
                )
                return 5
            if task["status"] == "DONE":
                result = task.get("result") or {}
                print(f"flamegraph={result.get('flamegraph_url')}")
                print(f"topn={result.get('topn_url')}")
            return 0
        time.sleep(2)

    print("timed out waiting for task completion", file=sys.stderr)
    return 3


if __name__ == "__main__":
    try:
        raise SystemExit(main())
    except urllib.error.HTTPError as exc:
        print(exc.read().decode("utf-8"), file=sys.stderr)
        raise
