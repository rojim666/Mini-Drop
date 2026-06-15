import argparse
import os
import subprocess
import sys
import time
from dataclasses import dataclass
from datetime import datetime, timezone
from pathlib import Path


ROOT = Path(__file__).resolve().parents[2]
DEFAULT_OUTPUT = ROOT / "artifacts" / "real-smoke-report.md"
DEFAULT_COLLECTORS = os.environ.get("COLLECTOR_TYPE") or "perf"
CHECKS = {
    "perf": "check_perf_env.py",
    "ebpf-syscall": "check_ebpf_env.py",
    "py-spy": "check_pyspy_env.py",
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
    preflight: CommandResult
    smoke: CommandResult | None

    @property
    def status(self) -> str:
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
    lines.extend(markdown_table(["Collector", "Status", "Preflight", "Smoke"], rows))
    lines.extend(
        [
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
        help="Comma-separated collectors to validate. Defaults to COLLECTOR_TYPE or perf.",
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
    for collector in collectors:
        preflight = run_preflight(collector, target_pid, env)
        smoke = None
        if preflight.ok and not args.skip_smoke:
            smoke = run_smoke(collector, target_pid, args.api_base, args.agent_id, env)
        results.append(CollectorResult(name=collector, preflight=preflight, smoke=smoke))

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
