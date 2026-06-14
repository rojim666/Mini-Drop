import argparse
import json
import os
import subprocess
import sys
import urllib.request
from datetime import datetime, timezone
from pathlib import Path


ROOT = Path(__file__).resolve().parents[2]
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


def run_git(args: list[str]) -> str:
    result = subprocess.run(
        ["git", *args],
        cwd=ROOT,
        check=False,
        capture_output=True,
        text=True,
        encoding="utf-8",
        errors="replace",
    )
    if result.returncode != 0:
        return result.stderr.strip() or result.stdout.strip() or f"git {' '.join(args)} failed"
    return result.stdout.strip()


def signed_url_ok(url: str) -> bool:
    return f"localhost:{MINIO_PORT}" in url and "X-Amz-Signature=" in url


def top_hotspot(result: dict) -> str:
    hotspots = result.get("hotspots") or []
    if not hotspots:
        return "-"
    top = hotspots[0]
    function = top.get("function", "-")
    percent = top.get("percent", 0)
    return f"{function} ({percent}%)"


def markdown_table(headers: list[str], rows: list[list[str]]) -> list[str]:
    lines = [
        "| " + " | ".join(headers) + " |",
        "| " + " | ".join("---" for _ in headers) + " |",
    ]
    for row in rows:
        lines.append("| " + " | ".join(cell.replace("|", "\\|") for cell in row) + " |")
    return lines


def collect_evidence() -> tuple[list[str], bool]:
    failures: list[str] = []
    generated_at = datetime.now(timezone.utc).strftime("%Y-%m-%d %H:%M:%S UTC")
    web_url = f"http://localhost:{WEB_PORT}"

    health = request_json("/healthz")
    if health.get("status") != "ok":
        failures.append(f"API health expected ok, got {health}")

    web_ok, web_status = check_url(web_url)
    if not web_ok:
        failures.append(f"Web UI not reachable on {web_url}: {web_status}")

    agents = request_json("/api/v1/agents").get("agents", [])
    online_agents = [agent for agent in agents if agent.get("status") == "ONLINE"]
    if not online_agents:
        failures.append("No ONLINE agent found")

    tasks = request_json("/api/v1/tasks").get("tasks", [])
    done_tasks = [task for task in tasks if task.get("status") == "DONE"]
    failed_tasks = [task for task in tasks if task.get("status") == "FAILED"]
    if not done_tasks:
        failures.append("No DONE task found")

    task_rows: list[list[str]] = []
    signed_url_count = 0
    comparable_count = 0
    for task in done_tasks[:8]:
        task_id = task.get("id", "")
        if not task_id:
            continue
        detail = request_json(f"/api/v1/tasks/{task_id}").get("task", {})
        result = detail.get("result") or {}
        has_hotspots = bool(result.get("hotspots"))
        if has_hotspots:
            comparable_count += 1
        flamegraph_url = result.get("flamegraph_url") or ""
        topn_url = result.get("topn_url") or ""
        has_signed_urls = signed_url_ok(flamegraph_url) and signed_url_ok(topn_url)
        if has_signed_urls:
            signed_url_count += 1
        task_rows.append(
            [
                task_id,
                str(detail.get("collector_type") or task.get("collector_type") or "-"),
                str(detail.get("target_pid") or task.get("target_pid") or "-"),
                str(detail.get("status") or task.get("status") or "-"),
                "yes" if has_signed_urls else "no",
                top_hotspot(result),
            ]
        )

    if signed_url_count == 0:
        failures.append(f"No DONE task has MinIO signed flamegraph/topn URLs on localhost:{MINIO_PORT}")
    if comparable_count < 2:
        failures.append("Task comparison needs at least two DONE tasks with TopN hotspots")

    branch = run_git(["branch", "--show-current"]) or "(detached)"
    head = run_git(["rev-parse", "--short", "HEAD"])
    commits = run_git(["log", "--oneline", "-n", "8"]).splitlines()
    status_lines = run_git(["status", "--short"]).splitlines()

    lines = [
        "# Mini-Drop Demo Evidence",
        "",
        f"Generated: {generated_at}",
        "",
        "## Runtime",
        "",
        f"- API: `{API_BASE}`",
        f"- Web: `{web_url}`",
        f"- MinIO public port: `{MINIO_PORT}`",
        f"- API health: `{health.get('status')}`",
        f"- Web reachable: `{web_ok}` ({web_status})",
        "",
        "## Acceptance Snapshot",
        "",
        f"- Agents online: `{len(online_agents)}/{len(agents)}`",
        f"- Online agent ids: `{', '.join(agent.get('id', '') for agent in online_agents) or '-'}`",
        f"- DONE tasks: `{len(done_tasks)}`",
        f"- FAILED tasks: `{len(failed_tasks)}`",
        f"- MinIO signed DONE results: `{signed_url_count}`",
        f"- Compare-ready tasks: `{comparable_count}`",
        f"- Acceptance: `{'FAILED' if failures else 'OK'}`",
        "",
        "## Completed Task Samples",
        "",
    ]
    if task_rows:
        lines.extend(markdown_table(["Task", "Collector", "PID", "Status", "Signed URLs", "Top hotspot"], task_rows))
    else:
        lines.append("_No completed task samples were available._")

    lines.extend(
        [
            "",
            "## Git State",
            "",
            f"- Branch: `{branch}`",
            f"- Head: `{head}`",
            "",
            "Recent commits:",
            "",
            "```text",
            "\n".join(commits) if commits else "(none)",
            "```",
            "",
            "Working tree status:",
            "",
            "```text",
            "\n".join(status_lines) if status_lines else "clean",
            "```",
            "",
            "## Verification Notes",
            "",
            "- This evidence file is generated from the live API/Web endpoints and the local Git worktree.",
            "- Real Linux collector smoke tests still require WSL2 / Linux host prerequisites.",
        ]
    )

    if failures:
        lines.extend(["", "## Failures", ""])
        lines.extend(f"- {failure}" for failure in failures)

    return lines, not failures


def main() -> int:
    parser = argparse.ArgumentParser(description="Write a Markdown evidence file for the Mini-Drop demo.")
    parser.add_argument(
        "--output",
        default=str(ROOT / "artifacts" / "demo-evidence.md"),
        help="Output Markdown path. Defaults to artifacts/demo-evidence.md.",
    )
    args = parser.parse_args()

    try:
        lines, ok = collect_evidence()
    except Exception as exc:
        print(f"collect evidence failed: {exc}", file=sys.stderr)
        return 1

    output_path = Path(args.output)
    if not output_path.is_absolute():
        output_path = ROOT / output_path
    output_path.parent.mkdir(parents=True, exist_ok=True)
    output_path.write_text("\n".join(lines) + "\n", encoding="utf-8")
    print(f"Wrote demo evidence to {output_path}")
    return 0 if ok else 1


if __name__ == "__main__":
    raise SystemExit(main())
