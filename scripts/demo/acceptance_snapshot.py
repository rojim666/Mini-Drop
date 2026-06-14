import json
import os
import sys
import urllib.error
import urllib.request


API_PORT = os.environ.get("MINIDROP_API_PORT", "8080")
API_BASE = os.environ.get("MINIDROP_API_BASE_URL", f"http://127.0.0.1:{API_PORT}").rstrip("/")
WEB_PORT = os.environ.get("MINIDROP_WEB_PORT", "4173")
MINIO_PORT = os.environ.get("MINIDROP_MINIO_PORT", "9000")


def request_json(path: str) -> dict:
    req = urllib.request.Request(f"{API_BASE}{path}", method="GET")
    with urllib.request.urlopen(req, timeout=10) as response:
        return json.loads(response.read().decode("utf-8"))


def check_url(url: str) -> tuple[bool, str]:
    try:
        req = urllib.request.Request(url, method="GET")
        with urllib.request.urlopen(req, timeout=10) as response:
            return 200 <= response.status < 400, str(response.status)
    except Exception as exc:
        return False, str(exc)


def signed_url_ok(url: str) -> bool:
    return f"localhost:{MINIO_PORT}" in url and "X-Amz-Signature=" in url


def main() -> int:
    failures: list[str] = []

    health = request_json("/healthz")
    if health.get("status") != "ok":
        failures.append(f"API health expected ok, got {health}")

    web_ok, web_status = check_url(f"http://localhost:{WEB_PORT}")
    if not web_ok:
        failures.append(f"Web UI not reachable on localhost:{WEB_PORT}: {web_status}")

    agents = request_json("/api/v1/agents").get("agents", [])
    online_agents = [agent for agent in agents if agent.get("status") == "ONLINE"]
    if not online_agents:
        failures.append("No ONLINE agent found")

    tasks = request_json("/api/v1/tasks").get("tasks", [])
    done_tasks = [task for task in tasks if task.get("status") == "DONE"]
    failed_tasks = [task for task in tasks if task.get("status") == "FAILED"]
    if not done_tasks:
        failures.append("No DONE task found")

    comparable_count = 0
    signed_url_count = 0
    done_task_ids: list[str] = []
    for task in done_tasks[:5]:
        task_id = task.get("id")
        if not task_id:
            continue
        done_task_ids.append(task_id)
        detail = request_json(f"/api/v1/tasks/{task_id}").get("task", {})
        result = detail.get("result") or {}
        hotspots = result.get("hotspots") or []
        if hotspots:
            comparable_count += 1
        flamegraph_url = result.get("flamegraph_url") or ""
        topn_url = result.get("topn_url") or ""
        if signed_url_ok(flamegraph_url) and signed_url_ok(topn_url):
            signed_url_count += 1

    if signed_url_count == 0:
        failures.append(f"No DONE task has MinIO signed flamegraph/topn URLs on localhost:{MINIO_PORT}")
    if comparable_count < 2:
        failures.append("Task comparison needs at least two DONE tasks with TopN hotspots")

    profiles = request_json("/api/v1/continuous-profiles").get("profiles", [])
    enabled_profiles = [profile for profile in profiles if profile.get("enabled")]
    trend_ready_profiles = 0
    profile_summaries: list[str] = []
    for profile in profiles[:5]:
        profile_id = profile.get("id")
        if not profile_id:
            continue
        windows_payload = request_json(f"/api/v1/continuous-profiles/{profile_id}/windows?limit=24")
        summary = windows_payload.get("summary") or {}
        trend = request_json(f"/api/v1/continuous-profiles/{profile_id}/trends?limit=12")
        series = trend.get("series") or []
        if series:
            trend_ready_profiles += 1
        profile_summaries.append(
            f"{profile_id}:{summary.get('done_windows', 0)}/{summary.get('total_windows', 0)}:{summary.get('latest_status', '-')}:trends={len(series)}"
        )

    print("Mini-Drop acceptance snapshot")
    print(f"api={API_BASE} health={health.get('status')}")
    print(f"web=http://localhost:{WEB_PORT} reachable={web_ok}")
    print(f"agents_online={len(online_agents)}/{len(agents)} ids={','.join(agent.get('id', '') for agent in online_agents)}")
    print(f"tasks_done={len(done_tasks)} tasks_failed={len(failed_tasks)} sampled_done={','.join(done_task_ids)}")
    print(f"minio_signed_results={signed_url_count}")
    print(f"compare_ready_tasks={comparable_count}")
    print(f"continuous_profiles={len(profiles)} enabled={len(enabled_profiles)} trend_ready={trend_ready_profiles}")
    print(f"continuous_profile_samples={','.join(profile_summaries) if profile_summaries else '-'}")

    if failures:
        print("acceptance=FAILED")
        for failure in failures:
            print(f"- {failure}", file=sys.stderr)
        return 1

    print("acceptance=OK")
    return 0


if __name__ == "__main__":
    try:
        raise SystemExit(main())
    except (urllib.error.URLError, urllib.error.HTTPError, RuntimeError) as exc:
        print(str(exc), file=sys.stderr)
        raise SystemExit(1)
