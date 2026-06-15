import argparse
import platform
import shutil
import subprocess
from pathlib import Path

from env_check_common import print_next_steps


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Check whether the Mini-Drop py-spy collector can run here.")
    parser.add_argument("--pid", type=int, default=0, help="Optional Python target PID to validate.")
    return parser.parse_args()


def fail(message: str, hint: str = "") -> int:
    print(f"FAIL: {message}")
    if hint:
        print(f"hint: {hint}")
    return 1


def ok(message: str) -> None:
    print(f"OK: {message}")


def check_pyspy() -> int:
    path = shutil.which("py-spy")
    if path is None:
        return fail("py-spy command not found", "Install it with: python -m pip install py-spy")
    ok(f"py-spy command found at {path}")
    return 0


def check_pid(pid: int) -> int:
    if pid <= 0:
        return 0

    if platform.system().lower() == "windows":
        completed = subprocess.run(
            ["tasklist", "/FI", f"PID eq {pid}", "/FO", "CSV", "/NH"],
            check=False,
            capture_output=True,
            text=True,
        )
        output = completed.stdout.strip()
        if not output or "No tasks are running" in output or "INFO:" in output:
            return fail(f"target pid {pid} not found")
        ok(f"target pid {pid} exists")
        return 0

    if not (Path("/proc") / str(pid)).exists():
        return fail(f"target pid {pid} not found", "Start scripts/demo/mock_target.py and use its printed target_pid.")
    ok(f"target pid {pid} exists")
    return 0


def check_pyspy_smoke(pid: int) -> int:
    if pid <= 0 or shutil.which("py-spy") is None:
        return 0

    try:
        completed = subprocess.run(
            ["py-spy", "dump", "--pid", str(pid)],
            check=False,
            capture_output=True,
            text=True,
            timeout=8,
        )
    except subprocess.TimeoutExpired:
        return fail("py-spy dump timed out")

    if completed.returncode != 0:
        details = (completed.stderr or completed.stdout).strip()
        return fail(f"py-spy dump failed: {details}", "Confirm the PID is a Python process and permissions allow attach.")

    ok("py-spy dump command completed")
    return 0


def main() -> int:
    args = parse_args()
    checks = [check_pyspy(), check_pid(args.pid), check_pyspy_smoke(args.pid)]
    if any(code != 0 for code in checks):
        print_next_steps(
            "py-spy collector",
            [
                "python3 -m pip install --user py-spy",
                "COLLECTOR_TYPE=py-spy bash ./scripts/demo/start-local.sh",
                "make smoke-real COLLECTOR_TYPE=py-spy",
            ],
        )
        return 1

    print("py-spy collector prerequisites look ready.")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
