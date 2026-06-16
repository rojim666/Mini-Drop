import argparse
import json
import os
import subprocess
import sys
import time
import urllib.error
import urllib.request
from dataclasses import dataclass
from datetime import datetime, timezone
from pathlib import Path

from api_auth import auth_headers


ROOT = Path(__file__).resolve().parents[2]
DEFAULT_OUTPUT = ROOT / "artifacts" / "real-smoke-report.md"
DEFAULT_COLLECTORS = os.environ.get("MINIDROP_REAL_COLLECTORS") or os.environ.get("COLLECTOR_TYPE") or "perf,ebpf-syscall,py-spy"
CHECKS = {
    "perf": "check_perf_env.py",
    "ebpf-syscall": "check_ebpf_env.py",
    "py-spy": "check_pyspy_env.py",
}
PRECHECK_HINTS = {
    "api": "Start the local Linux stack first: COLLECTOR_TYPE=perf bash ./scripts/demo/start-local.sh",
    "agent": "Wait for the Agent heartbeat or verify MINIDROP_AGENT_ID matches the running local Agent.",
    "pid": "Start the local demo target or set MINIDROP_TARGET_PID=<pid>.",
}


@dataclass
class CommandResult:
    name: str
    command: list[str]
    code: int
    output: str
    elapsed_sec: float

    @property
    def ok(self) -> bool:
        return self.code == 0


@dataclass
class CollectorResult:
    name: str
    api: CommandResult
    agent: CommandResult
    pid: CommandResult
    preflight: CommandResult
    smoke: CommandResult | None

    @property
    def status(self) -> str:
        for check in [self.api, self.agent, self.pid]:
            if not check.ok:
                return "BLOCKED"
        if not self.preflight.ok:
            return "BLOCKED"
        if self.smoke is None:
            return "READY"
        return "DONE" if self.smoke.ok else "FAILED"


def quote_command(args: list[str]) -> str:
    return " ".join(f'"{item}"' if any(char.isspace() for char in item) else item for item in args)


def run_command(name: str, args: list[str], env: dict[str, str], timeout: int) -> CommandResult:
    started = time.monotonic()
    try:
        completed = subprocess.run(
            args,
            cwd=ROOT,
            env=env,
            check=False,
            capture_output=True,
            text=True,
            encoding="utf-8",
            errors="replace",
            timeout=timeout,
        )
        output = "\n".join(part for part in [completed.stdout.strip(), completed.stderr.strip()] if part)
        code = completed.returncode
    except FileNotFoundError as exc:
        output = str(exc)
        code = 127
    except subprocess.TimeoutExpired as exc:
        output = "\n".join(part.strip() for part in [exc.stdout or "", exc.stderr or ""] if part.strip())
        if not output:
            output = f"{quote_command(args)} timed out after {timeout}s"
        code = 124

    return CommandResult(
        name=name,
        command=args,
        code=code,
        output=output or "(no output)",
        elapsed_sec=time.monotonic() - started,
    )


def parse_collectors(value: str) -> list[str]:
    collectors = [item.strip() for item in value.split(",") if item.strip()]
    unknown = [item for item in collectors if item not in CHECKS]
    if unknown:
        supported = ", ".join(CHECKS)
        raise ValueError(f"unsupported collectors: {', '.join(unknown)}; supported collectors: {supported}")
    return collectors


def read_target_pid(default_pid: int) -> int:
    if default_pid > 0:
        return default_pid
    pid_file = ROOT / "tmp" / "local-demo" / "target.pid"
    try:
        return int(pid_file.read_text(encoding="utf-8").strip())
    except (OSError, ValueError):
        return 0


def precheck_result(name: str, code: int, output: str) -> CommandResult:
    return CommandResult(
        name=name,
        command=["internal", name],
        code=code,
        output=output,
        elapsed_sec=0,
    )


def request_json(api_base: str, path: str, auth: bool) -> dict:
    headers = auth_headers(api_base) if auth else {}
    req = urllib.request.Request(f"{api_base.rstrip('/')}{path}", method="GET", headers=headers)
    with urllib.request.urlopen(req, timeout=10) as response:
        return json.loads(response.read().decode("utf-8"))


def check_api(api_base: str) -> CommandResult:
    started = time.monotonic()
    try:
        payload = request_json(api_base, "/healthz", auth=False)
        status = payload.get("status")
        if status != "ok":
            return CommandResult("API health precheck", ["GET", f"{api_base.rstrip('/')}/healthz"], 1, f"unexpected health payload: {payload}", time.monotonic() - started)
        return CommandResult("API health precheck", ["GET", f"{api_base.rstrip('/')}/healthz"], 0, "API health is ok", time.monotonic() - started)
    except (RuntimeError, urllib.error.URLError, urllib.error.HTTPError, TimeoutError) as exc:
        return CommandResult("API health precheck", ["GET", f"{api_base.rstrip('/')}/healthz"], 1, f"API is not reachable at {api_base}: {exc}\nhint: {PRECHECK_HINTS['api']}", time.monotonic() - started)


def check_agent(api_base: str, agent_id: str) -> CommandResult:
    started = time.monotonic()
    try:
        agents = request_json(api_base, "/api/v1/agents", auth=True).get("agents", [])
    except (RuntimeError, urllib.error.URLError, urllib.error.HTTPError, TimeoutError) as exc:
        return CommandResult("Agent online precheck", ["GET", f"{api_base.rstrip('/')}/api/v1/agents"], 1, f"cannot list agents from {api_base}: {exc}\nhint: {PRECHECK_HINTS['agent']}", time.monotonic() - started)

    observed = []
    for agent in agents:
        observed.append(f"{agent.get('id', '<unknown>')}:{agent.get('status', '<unknown>')}")
        if agent.get("id") == agent_id and agent.get("status") == "ONLINE":
            return CommandResult("Agent online precheck", ["GET", f"{api_base.rstrip('/')}/api/v1/agents"], 0, f"agent {agent_id} is ONLINE", time.monotonic() - started)
    observed_text = ", ".join(observed) if observed else "none"
    return CommandResult(
        "Agent online precheck",
        ["GET", f"{api_base.rstrip('/')}/api/v1/agents"],
        1,
        f"agent {agent_id} is not ONLINE; observed agents: {observed_text}\nhint: {PRECHECK_HINTS['agent']}",
        time.monotonic() - started,
    )


def check_pid_value(target_pid: int, skip_smoke: bool) -> CommandResult:
    if skip_smoke:
        return precheck_result("Target PID precheck", 0, "target PID is not required while --skip-smoke is set")
    if target_pid <= 0:
        return precheck_result("Target PID precheck", 1, f"target PID is missing\nhint: {PRECHECK_HINTS['pid']}")
    return precheck_result("Target PID precheck", 0, f"target PID {target_pid} will be passed to collector preflight and smoke")


def run_preflight(collector: str, target_pid: int, env: dict[str, str]) -> CommandResult:
    args = [sys.executable, str(ROOT / "scripts" / "demo" / CHECKS[collector])]
    if target_pid > 0:
        args.extend(["--pid", str(target_pid)])
    return run_command(f"{collector} preflight", args, env=env, timeout=90)


def run_smoke(collector: str, target_pid: int, api_base: str, agent_id: str, env: dict[str, str]) -> CommandResult:
    smoke_env = env.copy()
    smoke_env.update(
        {
            "COLLECTOR_TYPE": collector,
            "MINIDROP_API_BASE_URL": api_base,
            "MINIDROP_AGENT_ID": agent_id,
        }
    )
    if target_pid > 0:
        smoke_env["MINIDROP_TARGET_PID"] = str(target_pid)
    return run_command(
        f"{collector} smoke",
        ["bash", str(ROOT / "scripts" / "demo" / "smoke_real_collector.sh"), collector],
        env=smoke_env,
        timeout=180,
    )


def classify_failure(output: str) -> str:
    text = output.lower()
    if "api is not reachable" in text or "connection refused" in text or "healthz" in text:
        return "API not reachable"
    if "agent" in text and ("not online" in text or "observed agents" in text):
        return "Agent not online"
    if "target pid" in text and ("not found" in text or "missing" in text):
        return "Target PID missing"
    if "requires linux" in text:
        return "Linux runtime required"
    if "command not found" in text or "no such file or directory" in text:
        return "Collector tool missing"
    if "permission" in text or "paranoid" in text or "tracefs" in text or "ptrace" in text:
        return "Collector permission blocked"
    if "timed out" in text or "timeout" in text:
        return "Collector timed out"
    if "smoke failed" in text or "expected status" in text:
        return "Collector smoke failed"
    return "See command output"


def collector_reason(item: CollectorResult) -> str:
    checks = [item.api, item.agent, item.pid, item.preflight]
    for check in checks:
        if not check.ok:
            return classify_failure(check.output)
    if item.smoke is not None and not item.smoke.ok:
        return classify_failure(item.smoke.output)
    if item.status == "READY":
        return "Preflight passed; smoke skipped"
    if item.status == "DONE":
        return "Smoke passed"
    return "See command output"


def truncate(output: str, limit: int = 7000) -> str:
    if len(output) <= limit:
        return output
    head = output[: limit // 2].rstrip()
    tail = output[-limit // 2 :].lstrip()
    return f"{head}\n\n... output truncated ...\n\n{tail}"


def markdown_table(headers: list[str], rows: list[list[str]]) -> list[str]:
    lines = [
        "| " + " | ".join(headers) + " |",
        "| " + " | ".join("---" for _ in headers) + " |",
    ]
    for row in rows:
        lines.append("| " + " | ".join(cell.replace("|", "\\|") for cell in row) + " |")
    return lines


def write_report(output_path: Path, collectors: list[CollectorResult], target_pid: int, api_base: str, agent_id: str) -> None:
    generated_at = datetime.now(timezone.utc).strftime("%Y-%m-%d %H:%M:%S UTC")
    rows = [
        [
            item.name,
            item.status,
            collector_reason(item),
            "OK" if item.api.ok else f"FAILED ({item.api.code})",
            "OK" if item.agent.ok else f"FAILED ({item.agent.code})",
            "OK" if item.pid.ok else f"FAILED ({item.pid.code})",
            "OK" if item.preflight.ok else f"FAILED ({item.preflight.code})",
            "-" if item.smoke is None else ("OK" if item.smoke.ok else f"FAILED ({item.smoke.code})"),
        ]
        for item in collectors
    ]
    statuses = {item.status for item in collectors}
    overall = "BLOCKED"
    if statuses == {"DONE"}:
        overall = "DONE"
    elif statuses == {"READY"}:
        overall = "READY"
    elif "FAILED" in statuses:
        overall = "FAILED"

    lines = [
        "# Mini-Drop Real Smoke Report",
        "",
        f"Generated: {generated_at}",
        "",
        "## Runtime Inputs",
        "",
        f"- Target PID: `{target_pid if target_pid > 0 else 'not provided'}`",
        f"- API base: `{api_base}`",
        f"- Agent ID: `{agent_id}`",
        f"- Overall: `{overall}`",
        "",
        "## Summary",
        "",
    ]
    lines.extend(markdown_table(["Collector", "Status", "Reason", "API", "Agent", "PID", "Preflight", "Smoke"], rows))
    lines.extend(
        [
            "",
            "## Status Meaning",
            "",
            "- `BLOCKED`: preflight cannot start because API, Agent, PID, OS tools, or permissions are missing.",
            "- `READY`: preflight passed and smoke was intentionally skipped.",
            "- `DONE`: preflight and real smoke task completed.",
            "- `FAILED`: preflight passed, but the real smoke task failed.",
            "",
            "## How To Unblock",
            "",
            "```bash",
            "make real-preflight",
            "make real-check",
            "COLLECTOR_TYPE=perf bash ./scripts/demo/start-local.sh",
            "make smoke-real COLLECTOR_TYPE=perf",
            "```",
            "",
            "## Command Output",
            "",
        ]
    )

    shared_checks: list[CommandResult] = []
    if collectors:
        shared_checks = [collectors[0].api, collectors[0].agent, collectors[0].pid]

    for result in shared_checks:
        lines.extend(
            [
                f"### {result.name}",
                "",
                f"- Exit code: `{result.code}`",
                f"- Elapsed: `{result.elapsed_sec:.1f}s`",
                f"- Command: `{quote_command(result.command)}`",
                "",
                "```text",
                truncate(result.output),
                "```",
                "",
            ]
        )

    for item in collectors:
        for result in [item.preflight, item.smoke]:
            if result is None:
                continue
            lines.extend(
                [
                    f"### {result.name}",
                    "",
                    f"- Exit code: `{result.code}`",
                    f"- Elapsed: `{result.elapsed_sec:.1f}s`",
                    f"- Command: `{quote_command(result.command)}`",
                    "",
                    "```text",
                    truncate(result.output),
                    "```",
                    "",
                ]
            )

    output_path.parent.mkdir(parents=True, exist_ok=True)
    output_path.write_text("\n".join(lines) + "\n", encoding="utf-8")


def main() -> int:
    parser = argparse.ArgumentParser(description="Write a Mini-Drop real collector smoke report.")
    parser.add_argument(
        "--collectors",
        default=DEFAULT_COLLECTORS,
        help="Comma-separated collectors to validate. Defaults to MINIDROP_REAL_COLLECTORS, COLLECTOR_TYPE, or perf,ebpf-syscall,py-spy.",
    )
    parser.add_argument("--pid", type=int, default=int(os.environ.get("MINIDROP_TARGET_PID", "0") or "0"))
    parser.add_argument("--api-base", default=os.environ.get("MINIDROP_API_BASE_URL", "http://127.0.0.1:8080"))
    parser.add_argument("--agent-id", default=os.environ.get("MINIDROP_AGENT_ID", "agt_local"))
    parser.add_argument("--output", default=str(DEFAULT_OUTPUT))
    parser.add_argument("--skip-smoke", action="store_true", help="Only run collector preflight checks.")
    parser.add_argument(
        "--allow-blocked",
        action="store_true",
        help="Return success when collectors are blocked, as long as the report was written.",
    )
    args = parser.parse_args()

    try:
        collectors = parse_collectors(args.collectors)
    except ValueError as exc:
        print(str(exc), file=sys.stderr)
        return 2

    target_pid = args.pid if args.skip_smoke else read_target_pid(args.pid)
    env = os.environ.copy()
    results: list[CollectorResult] = []
    if args.skip_smoke:
        shared_api_check = precheck_result("API health precheck", 0, "API health is not required while --skip-smoke is set")
        shared_agent_check = precheck_result("Agent online precheck", 0, "Agent online status is not required while --skip-smoke is set")
    else:
        shared_api_check = check_api(args.api_base)
        shared_agent_check = check_agent(args.api_base, args.agent_id) if shared_api_check.ok else precheck_result(
            "Agent online precheck",
            1,
            f"skipped because API health failed\nhint: {PRECHECK_HINTS['api']}",
        )
    shared_pid_check = check_pid_value(target_pid, args.skip_smoke)
    for collector in collectors:
        preflight_pid = target_pid if shared_pid_check.ok else 0
        preflight = run_preflight(collector, preflight_pid, env)
        smoke = None
        if shared_api_check.ok and shared_agent_check.ok and shared_pid_check.ok and preflight.ok and not args.skip_smoke:
            smoke = run_smoke(collector, target_pid, args.api_base, args.agent_id, env)
        results.append(
            CollectorResult(
                name=collector,
                api=shared_api_check,
                agent=shared_agent_check,
                pid=shared_pid_check,
                preflight=preflight,
                smoke=smoke,
            )
        )

    output_path = Path(args.output)
    if not output_path.is_absolute():
        output_path = ROOT / output_path
    write_report(output_path, results, target_pid, args.api_base.rstrip("/"), args.agent_id)

    print(f"Wrote real smoke report to {output_path}")
    failed = [item for item in results if item.status in {"BLOCKED", "FAILED"}]
    if failed and not args.allow_blocked:
        print("real_smoke=BLOCKED_OR_FAILED")
        return 1
    print("real_smoke=OK")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
