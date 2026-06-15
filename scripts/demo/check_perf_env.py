import argparse
import os
import shutil
import subprocess
import sys
from pathlib import Path

from env_check_common import is_linux, print_next_steps, runtime_label


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Check whether the Mini-Drop perf collector can run here.")
    parser.add_argument("--pid", type=int, default=0, help="Optional target PID to validate.")
    return parser.parse_args()


def fail(message: str, hint: str = "") -> int:
    print(f"FAIL: {message}")
    if hint:
        print(f"hint: {hint}")
    return 1


def ok(message: str) -> None:
    print(f"OK: {message}")


def check_linux() -> int:
    if not is_linux():
        return fail("perf collector requires linux", "Run this check inside WSL2 Ubuntu or another Linux host.")
    ok(runtime_label())
    return 0


def check_perf() -> int:
    perf_path = shutil.which("perf")
    if perf_path is None:
        return fail(
            "perf command not found",
            "Install it with: sudo apt-get install linux-tools-common linux-tools-generic",
        )
    ok(f"perf command found at {perf_path}")
    return 0


def check_paranoid() -> int:
    paranoid_path = Path("/proc/sys/kernel/perf_event_paranoid")
    if not paranoid_path.exists():
        ok("perf_event_paranoid is not present on this kernel")
        return 0

    raw_value = paranoid_path.read_text(encoding="utf-8").strip()
    try:
        value = int(raw_value)
    except ValueError:
        return fail(f"cannot parse perf_event_paranoid={raw_value!r}")

    if value > 1:
        return fail(
            f"perf_event_paranoid={value} blocks process profiling",
            "For this demo session run: sudo sysctl kernel.perf_event_paranoid=1",
        )

    ok(f"perf_event_paranoid={value} allows process profiling")
    return 0


def check_pid(pid: int) -> int:
    if pid <= 0:
        return 0

    proc_path = Path("/proc") / str(pid)
    if not proc_path.exists():
        return fail(f"target pid {pid} not found", "Start scripts/demo/mock_target.py and use its printed target_pid.")

    ok(f"target pid {pid} exists")
    return 0


def check_perf_smoke(pid: int) -> int:
    if pid <= 0 or shutil.which("perf") is None:
        return 0

    try:
        completed = subprocess.run(
            ["perf", "record", "-F", "49", "-g", "-p", str(pid), "-o", os.devnull, "--", "sleep", "1"],
            check=False,
            capture_output=True,
            text=True,
            timeout=5,
        )
    except subprocess.TimeoutExpired:
        return fail("perf record smoke timed out")

    if completed.returncode != 0:
        details = (completed.stderr or completed.stdout).strip()
        return fail(f"perf record smoke failed: {details}")

    ok("perf record smoke command completed")
    return 0


def main() -> int:
    args = parse_args()
    checks = [
        check_linux(),
        check_perf(),
        check_paranoid(),
        check_pid(args.pid),
        check_perf_smoke(args.pid),
    ]
    if any(code != 0 for code in checks):
        print_next_steps(
            "perf collector",
            [
                "sudo apt-get update && sudo apt-get install -y linux-tools-common linux-tools-generic",
                "sudo sysctl kernel.perf_event_paranoid=1",
                "COLLECTOR_TYPE=perf bash ./scripts/demo/start-local.sh",
                "make smoke-real COLLECTOR_TYPE=perf",
            ],
        )
        return 1

    print("Perf collector prerequisites look ready.")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
