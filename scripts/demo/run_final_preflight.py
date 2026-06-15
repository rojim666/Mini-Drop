import argparse
import os
import subprocess
import sys
import time
from dataclasses import dataclass
from datetime import datetime, timezone
from pathlib import Path


ROOT = Path(__file__).resolve().parents[2]
DEFAULT_API_PORT = os.environ.get("MINIDROP_API_PORT", "8080")
DEFAULT_WEB_PORT = os.environ.get("MINIDROP_WEB_PORT", "4173")
DEFAULT_MINIO_PORT = os.environ.get("MINIDROP_MINIO_PORT", "9000")
DEFAULT_MINIO_CONSOLE_PORT = os.environ.get("MINIDROP_MINIO_CONSOLE_PORT", "9001")
DEFAULT_REAL_COLLECTORS = os.environ.get("MINIDROP_REAL_COLLECTORS", "perf,ebpf-syscall,py-spy")

REQUIRED_DOCS = [
    "docs/design/00-project-brief.md",
    "docs/design/01-mvp-scope.md",
    "docs/design/02-architecture.md",
    "docs/design/03-state-machines-and-observability.md",
    "docs/design/04-development-plan.md",
    "docs/design/05-backlog.md",
    "docs/design/06-next-implementation.md",
    "docs/design/07-attribution-evaluation.md",
    "docs/demo-runbook.md",
    "docs/demo-script.md",
]

PYTHON_SYNTAX_FILES = [
    "scripts/demo/acceptance_snapshot.py",
    "scripts/demo/write_demo_evidence.py",
    "scripts/demo/write_recording_checklist.py",
    "scripts/demo/write_submission_notes.py",
    "scripts/demo/write_real_smoke_report.py",
    "scripts/demo/capture_submission_artifacts.py",
    "scripts/demo/run_final_preflight.py",
    "scripts/demo/check_coverage.py",
    "scripts/demo/demo_diagnostics.py",
    "scripts/demo/check_compose_stack.py",
]


@dataclass
class StepResult:
    name: str
    command: str
    required: bool
    code: int
    output: str
    elapsed_sec: float

    @property
    def ok(self) -> bool:
        return self.code == 0


def executable(name: str) -> str:
    if os.name == "nt" and name in {"npm", "npx"}:
        return f"{name}.cmd"
    return name


def quote_command(args: list[str]) -> str:
    quoted: list[str] = []
    for arg in args:
        if not arg:
            quoted.append('""')
        elif any(char.isspace() for char in arg) or any(char in arg for char in ['"', "'"]):
            quoted.append('"' + arg.replace('"', '\\"') + '"')
        else:
            quoted.append(arg)
    return " ".join(quoted)


def run_command(name: str, args: list[str], required: bool, env: dict[str, str], timeout: int = 300) -> StepResult:
    started = time.monotonic()
    try:
        result = subprocess.run(
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
        output = "\n".join(part for part in [result.stdout.strip(), result.stderr.strip()] if part)
        code = result.returncode
    except FileNotFoundError as exc:
        output = str(exc)
        code = 127
    except subprocess.TimeoutExpired as exc:
        stdout = exc.stdout or ""
        stderr = exc.stderr or ""
        output = "\n".join(part.strip() for part in [stdout, stderr] if part)
        if not output:
            output = f"{quote_command(args)} timed out after {timeout}s"
        code = 124

    return StepResult(
        name=name,
        command=quote_command(args),
        required=required,
        code=code,
        output=output,
        elapsed_sec=time.monotonic() - started,
    )


def check_required_docs() -> StepResult:
    started = time.monotonic()
    missing = [path for path in REQUIRED_DOCS if not (ROOT / path).is_file()]
    if missing:
        output = "Missing required docs:\n" + "\n".join(f"- {path}" for path in missing)
        code = 1
    else:
        output = "Required docs exist:\n" + "\n".join(f"- {path}" for path in REQUIRED_DOCS)
        code = 0
    return StepResult(
        name="Required docs",
        command="internal docs existence check",
        required=True,
        code=code,
        output=output,
        elapsed_sec=time.monotonic() - started,
    )


def seed_acceptance_tasks(count: int, target_pid: int, agent_id: str, env: dict[str, str]) -> StepResult:
    started = time.monotonic()
    outputs: list[str] = []
    code = 0
    for index in range(1, count + 1):
        outputs.append(f"Seeding acceptance task {index}/{count}...")
        result = run_command(
            name=f"Seed acceptance task {index}",
            args=[
                sys.executable,
                str(ROOT / "scripts" / "demo" / "smoke_compose.py"),
                "--pid",
                str(target_pid),
                "--agent-id",
                agent_id,
                "--expect-minio-url",
            ],
            required=True,
            env=env,
            timeout=180,
        )
        outputs.append(result.output or "(no output)")
        if result.code != 0:
            code = result.code
            break

    return StepResult(
        name="Seed acceptance tasks",
        command=f"smoke_compose.py --expect-minio-url x{count}",
        required=True,
        code=code,
        output="\n\n".join(outputs),
        elapsed_sec=time.monotonic() - started,
    )


def seed_failure_task(agent_id: str, env: dict[str, str]) -> StepResult:
    return run_command(
        "Seed failure-path task",
        [
            sys.executable,
            str(ROOT / "scripts" / "demo" / "smoke_compose.py"),
            "--pid",
            "999999",
            "--agent-id",
            agent_id,
            "--expect-status",
            "FAILED",
            "--expect-reason-contains",
            "target pid not found",
        ],
        required=True,
        env=env,
        timeout=120,
    )


def check_real_collector_readiness(collectors: str, env: dict[str, str]) -> StepResult:
    ps_args = [
        "powershell",
        "-NoProfile",
        "-ExecutionPolicy",
        "Bypass",
        "-File",
        str(ROOT / "scripts" / "demo" / "prepare-real-collectors.ps1"),
        "-Collectors",
        collectors,
        "-Output",
        str(ROOT / "artifacts" / "real-collector-preflight.md"),
    ]

    direct_args = [
        sys.executable,
        str(ROOT / "scripts" / "demo" / "prepare_real_collectors.py"),
        "--collectors",
        collectors,
        "--output",
        str(ROOT / "artifacts" / "real-collector-preflight.md"),
    ]
    args = ps_args if os.name == "nt" else direct_args
    result = run_command("Real collector readiness", args, required=False, env=env, timeout=180)
    if result.code != 0:
        result.output = "\n".join(
            part
            for part in [
                result.output,
                "Real collectors are BLOCKED on this host. This is allowed for the Windows compose recording path when the generated report lists the missing Linux/WSL2 tools or permissions.",
            ]
            if part
        )
    return result


def check_real_smoke_report(collectors: str, env: dict[str, str]) -> StepResult:
    ps_args = [
        "powershell",
        "-NoProfile",
        "-ExecutionPolicy",
        "Bypass",
        "-File",
        str(ROOT / "scripts" / "demo" / "write-real-smoke-report.ps1"),
        "-Collectors",
        collectors,
        "-Output",
        str(ROOT / "artifacts" / "real-smoke-report.md"),
        "-SkipSmoke",
    ]
    direct_args = [
        sys.executable,
        str(ROOT / "scripts" / "demo" / "write_real_smoke_report.py"),
        "--collectors",
        collectors,
        "--output",
        str(ROOT / "artifacts" / "real-smoke-report.md"),
        "--skip-smoke",
        "--allow-blocked",
    ]

    args = ps_args if os.name == "nt" else direct_args
    result = run_command("Real smoke report", args, required=False, env=env, timeout=180)
    if result.code != 0:
        result.output = "\n".join(
            part
            for part in [
                result.output,
                "Real smoke report generation is non-blocking for the Windows compose recording path.",
            ]
            if part
        )
    return result


def truncate_output(output: str, limit: int = 6000) -> str:
    if len(output) <= limit:
        return output or "(no output)"
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


def write_report(
    output_path: Path,
    results: list[StepResult],
    api_port: str,
    web_port: str,
    minio_port: str,
    minio_console_port: str,
) -> None:
    generated_at = datetime.now(timezone.utc).strftime("%Y-%m-%d %H:%M:%S UTC")
    required_failed = [result for result in results if result.required and not result.ok]
    overall = "FAILED" if required_failed else "OK"
    rows = [
        [
            result.name,
            "yes" if result.required else "no",
            "OK" if result.ok else f"FAILED ({result.code})",
            f"{result.elapsed_sec:.1f}s",
        ]
        for result in results
    ]

    lines = [
        "# Mini-Drop Final Preflight",
        "",
        f"Generated: {generated_at}",
        "",
        "## Runtime Links",
        "",
        f"- API health: `http://localhost:{api_port}/healthz`",
        f"- Web console: `http://localhost:{web_port}`",
        f"- MinIO API: `http://localhost:{minio_port}`",
        f"- MinIO console: `http://localhost:{minio_console_port}`",
        "",
        "## Overall",
        "",
        f"- Status: `{overall}`",
        f"- Required failures: `{len(required_failed)}`",
        "",
        "## Step Summary",
        "",
    ]
    lines.extend(markdown_table(["Step", "Required", "Status", "Elapsed"], rows))

    if required_failed:
        lines.extend(["", "## Required Failures", ""])
        lines.extend(f"- `{result.name}` failed with exit code `{result.code}`." for result in required_failed)

    lines.extend(["", "## Step Output", ""])
    for result in results:
        lines.extend(
            [
                f"### {result.name}",
                "",
                f"- Required: `{'yes' if result.required else 'no'}`",
                f"- Exit code: `{result.code}`",
                f"- Command: `{result.command}`",
                "",
                "```text",
                truncate_output(result.output),
                "```",
                "",
            ]
        )

    lines.extend(
        [
            "## Recording Gate",
            "",
            "- Start recording only when the overall status is `OK`.",
            "- If real collector preflight is included, `BLOCKED` in the evidence file is acceptable on Windows only when the Linux/WSL2 commands and next steps are shown.",
            "- Keep `artifacts/demo-evidence.md`, `artifacts/coverage-report.md`, `artifacts/recording-checklist.md`, and `artifacts/submission-notes.md` open before the final walkthrough.",
        ]
    )

    output_path.parent.mkdir(parents=True, exist_ok=True)
    output_path.write_text("\n".join(lines) + "\n", encoding="utf-8")


def main() -> int:
    parser = argparse.ArgumentParser(description="Run the Mini-Drop final recording preflight.")
    parser.add_argument("--api-port", default=DEFAULT_API_PORT, help=f"API port. Defaults to {DEFAULT_API_PORT}.")
    parser.add_argument("--web-port", default=DEFAULT_WEB_PORT, help=f"Web port. Defaults to {DEFAULT_WEB_PORT}.")
    parser.add_argument("--minio-port", default=DEFAULT_MINIO_PORT, help=f"MinIO public port. Defaults to {DEFAULT_MINIO_PORT}.")
    parser.add_argument(
        "--minio-console-port",
        default=DEFAULT_MINIO_CONSOLE_PORT,
        help=f"MinIO console port. Defaults to {DEFAULT_MINIO_CONSOLE_PORT}.",
    )
    parser.add_argument(
        "--output",
        default=str(ROOT / "artifacts" / "final-preflight.md"),
        help="Output Markdown path. Defaults to artifacts/final-preflight.md.",
    )
    parser.add_argument("--seed-tasks", action="store_true", help="Create compose smoke tasks before acceptance snapshot.")
    parser.add_argument(
        "--seed-failure-task",
        action="store_true",
        default=True,
        help="Create a PID-not-found task so screenshot evidence includes the failure path.",
    )
    parser.add_argument(
        "--skip-failure-task",
        action="store_false",
        dest="seed_failure_task",
        help="Skip creation of the PID-not-found task during the live preflight.",
    )
    parser.add_argument(
        "--seed-task-count",
        type=int,
        default=int(os.environ.get("MINIDROP_ACCEPTANCE_SEED_TASKS", "2")),
        help="Number of smoke tasks to seed when --seed-tasks is set.",
    )
    parser.add_argument(
        "--target-pid",
        type=int,
        default=int(os.environ.get("MINIDROP_TARGET_PID", "1")),
        help="Target PID for seeded compose tasks.",
    )
    parser.add_argument(
        "--agent-id",
        default=os.environ.get("MINIDROP_TARGET_AGENT_ID", "agt_compose"),
        help="Agent ID for seeded compose tasks.",
    )
    parser.add_argument(
        "--include-real-preflight",
        action="store_true",
        help="Include real collector preflight in artifacts/demo-evidence.md.",
    )
    parser.add_argument(
        "--real-collectors",
        default=DEFAULT_REAL_COLLECTORS,
        help=f"Collectors passed to real preflight. Defaults to {DEFAULT_REAL_COLLECTORS}.",
    )
    parser.add_argument("--skip-live", action="store_true", help="Skip live API/Web acceptance and evidence checks.")
    parser.add_argument("--skip-tests", action="store_true", help="Skip Go, analyzer, and Web build verification.")
    args = parser.parse_args()

    if args.seed_tasks and args.seed_task_count < 1:
        print("--seed-task-count must be at least 1 when --seed-tasks is set", file=sys.stderr)
        return 2

    env = os.environ.copy()
    env.update(
        {
            "MINIDROP_API_PORT": str(args.api_port),
            "MINIDROP_WEB_PORT": str(args.web_port),
            "MINIDROP_MINIO_PORT": str(args.minio_port),
            "MINIDROP_MINIO_CONSOLE_PORT": str(args.minio_console_port),
            "MINIDROP_API_BASE_URL": env.get("MINIDROP_API_BASE_URL", f"http://127.0.0.1:{args.api_port}"),
        }
    )

    results: list[StepResult] = [check_required_docs()]
    results.append(
        run_command(
            "Python script syntax",
            [sys.executable, "-m", "py_compile", *PYTHON_SYNTAX_FILES],
            required=True,
            env=env,
            timeout=60,
        )
    )
    results.append(
        run_command("Git whitespace check", ["git", "diff", "--check"], required=True, env=env, timeout=60)
    )

    if args.include_real_preflight:
        results.append(check_real_collector_readiness(args.real_collectors, env))
        results.append(check_real_smoke_report(args.real_collectors, env))

    if not args.skip_tests:
        test_env = env.copy()
        test_env["GOPROXY"] = test_env.get("GOPROXY", "https://goproxy.cn,direct")
        results.append(
            run_command(
                "Go tests",
                ["go", "test", "./apps/api-server", "./apps/agent", "./internal/..."],
                required=True,
                env=test_env,
                timeout=180,
            )
        )
        results.append(
            run_command(
                "Analyzer tests",
                [sys.executable, "-m", "unittest", "apps.analyzer.main_test"],
                required=True,
                env=env,
                timeout=120,
            )
        )
        results.append(
            run_command(
                "Coverage gates",
                [sys.executable, str(ROOT / "scripts" / "demo" / "check_coverage.py")],
                required=True,
                env=test_env,
                timeout=240,
            )
        )
        results.append(
            run_command(
                "Web production build",
                [executable("npm"), "--prefix", str(ROOT / "apps" / "web"), "run", "build"],
                required=True,
                env=env,
                timeout=180,
            )
        )

    if not args.skip_live:
        results.append(
            run_command(
                "Compose stack health",
                [
                    sys.executable,
                    str(ROOT / "scripts" / "demo" / "check_compose_stack.py"),
                    "--api-port",
                    str(args.api_port),
                    "--web-port",
                    str(args.web_port),
                    "--minio-port",
                    str(args.minio_port),
                    "--minio-console-port",
                    str(args.minio_console_port),
                ],
                required=True,
                env=env,
                timeout=60,
            )
        )
        if args.seed_tasks:
            results.append(seed_acceptance_tasks(args.seed_task_count, args.target_pid, args.agent_id, env))
        if args.seed_failure_task:
            results.append(seed_failure_task(args.agent_id, env))
        results.append(
            run_command(
                "Acceptance snapshot",
                [sys.executable, str(ROOT / "scripts" / "demo" / "acceptance_snapshot.py")],
                required=True,
                env=env,
                timeout=90,
            )
        )
        evidence_args = [
            sys.executable,
            str(ROOT / "scripts" / "demo" / "write_demo_evidence.py"),
            "--output",
            str(ROOT / "artifacts" / "demo-evidence.md"),
        ]
        if args.include_real_preflight:
            evidence_args.extend(["--include-real-preflight", "--real-collectors", args.real_collectors])
        results.append(
            run_command("Demo evidence", evidence_args, required=True, env=env, timeout=180)
        )

    results.append(
        run_command(
            "Recording checklist",
            [
                sys.executable,
                str(ROOT / "scripts" / "demo" / "write_recording_checklist.py"),
                "--output",
                str(ROOT / "artifacts" / "recording-checklist.md"),
                "--api-port",
                str(args.api_port),
                "--web-port",
                str(args.web_port),
                "--minio-port",
                str(args.minio_port),
                "--minio-console-port",
                str(args.minio_console_port),
            ],
            required=True,
            env=env,
            timeout=60,
        )
    )
    results.append(
        run_command(
            "Submission notes",
            [
                sys.executable,
                str(ROOT / "scripts" / "demo" / "write_submission_notes.py"),
                "--output",
                str(ROOT / "artifacts" / "submission-notes.md"),
                "--api-port",
                str(args.api_port),
                "--web-port",
                str(args.web_port),
                "--minio-console-port",
                str(args.minio_console_port),
            ],
            required=True,
            env=env,
            timeout=60,
        )
    )
    if not args.skip_live:
        results.append(
            run_command(
                "Submission screenshots",
                [
                    sys.executable,
                    str(ROOT / "scripts" / "demo" / "capture_submission_artifacts.py"),
                    "--web-base",
                    f"http://localhost:{args.web_port}",
                    "--output-dir",
                    str(ROOT / "artifacts" / "submission-screenshots"),
                    "--minio-console-base",
                    f"http://localhost:{args.minio_console_port}",
                    "--evidence-path",
                    str(ROOT / "artifacts" / "demo-evidence.md"),
                    "--coverage-path",
                    str(ROOT / "artifacts" / "coverage-report.md"),
                    "--browser-channel",
                    "auto",
                ],
                required=True,
                env=env,
                timeout=300,
            )
        )

    output_path = Path(args.output)
    if not output_path.is_absolute():
        output_path = ROOT / output_path
    write_report(
        output_path,
        results,
        api_port=str(args.api_port),
        web_port=str(args.web_port),
        minio_port=str(args.minio_port),
        minio_console_port=str(args.minio_console_port),
    )

    failed = [result for result in results if result.required and not result.ok]
    print(f"Wrote final preflight report to {output_path}")
    if failed:
        print("final_preflight=FAILED")
        for result in failed:
            print(f"- {result.name} failed with exit code {result.code}", file=sys.stderr)
        return 1
    print("final_preflight=OK")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
