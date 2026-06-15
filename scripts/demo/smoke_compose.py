import json
import os
import sys
import time
import urllib.error
import urllib.request
from argparse import ArgumentParser


API_PORT = os.environ.get("MINIDROP_API_PORT", "8080")
API_BASE = os.environ.get("MINIDROP_API_BASE_URL", f"http://127.0.0.1:{API_PORT}").rstrip("/")
DEFAULT_AGENT_ID = os.environ.get("MINIDROP_TARGET_AGENT_ID", "agt_compose")
DEFAULT_TARGET_PID = int(os.environ.get("MINIDROP_TARGET_PID", "1"))
DEFAULT_MINIO_PORT = os.environ.get("MINIDROP_MINIO_PORT", "9000")


def parse_args() -> object:
    parser = ArgumentParser(description="Run a Mini-Drop Docker Compose smoke task.")
    parser.add_argument("--pid", type=int, default=DEFAULT_TARGET_PID)
    parser.add_argument("--agent-id", default=DEFAULT_AGENT_ID)
    parser.add_argument("--expect-status", choices=["DONE", "FAILED"], default="DONE")
    parser.add_argument("--expect-reason-contains", default="")
    parser.add_argument(
        "--expect-minio-url",
        action="store_true",
        default=os.environ.get("MINIDROP_EXPECT_MINIO_URL", "0") == "1",
    )
    return parser.parse_args()


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


def wait_for_agent(deadline: float, agent_id: str) -> None:
    observed_agents: list[dict] = []
    while time.time() < deadline:
        agents = request_json("/api/v1/agents").get("agents", [])
        observed_agents = agents
        for agent in agents:
            if agent.get("id") == agent_id and agent.get("status") == "ONLINE":
                return
        time.sleep(1)

    observed = ", ".join(
        f"{agent.get('id', '<unknown>')}:{agent.get('status', '<unknown>')}" for agent in observed_agents
    )
    if not observed:
        observed = "none"
    raise RuntimeError(
        f"agent {agent_id} did not become ONLINE at {API_BASE}; observed agents: {observed}. "
        "If you have a local API/Agent already running on the same port, stop it or set MINIDROP_API_PORT "
        "and MINIDROP_WEB_PORT before starting the compose stack."
    )


def create_and_wait_task(deadline: float, args: object) -> int:
    body = {
        "target_pid": args.pid,
        "target_agent_id": args.agent_id,
        "sample_duration_sec": 5,
        "sample_rate_hz": 99,
        "collector_type": "mock-perf",
    }
    created = request_json("/api/v1/tasks", method="POST", body=body)
    task_id = created["task"]["id"]
    print(f"created task {task_id}")

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
                flamegraph_url = result.get("flamegraph_url")
                topn_url = result.get("topn_url")
                print(f"flamegraph={flamegraph_url}")
                print(f"topn={topn_url}")
                if args.expect_minio_url:
                    expected_host = f"localhost:{DEFAULT_MINIO_PORT}"
                    urls = [flamegraph_url or "", topn_url or ""]
                    if not all(expected_host in item and "X-Amz-Signature=" in item for item in urls):
                        print(
                            f"expected MinIO signed URLs on {expected_host}, got flamegraph={flamegraph_url} topn={topn_url}",
                            file=sys.stderr,
                        )
                        return 6
            return 0
        time.sleep(2)

    print("timed out waiting for task completion", file=sys.stderr)
    return 3


def main() -> int:
    args = parse_args()
    deadline = time.time() + 90
    wait_for_api(deadline)
    wait_for_agent(deadline, args.agent_id)
    return create_and_wait_task(deadline, args)


if __name__ == "__main__":
    try:
        raise SystemExit(main())
    except (RuntimeError, urllib.error.URLError, urllib.error.HTTPError) as exc:
        print(str(exc), file=sys.stderr)
        raise SystemExit(1)
