import argparse
import shutil
import subprocess
from pathlib import Path

from env_check_common import is_linux, print_next_steps, runtime_label


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Check whether the Mini-Drop ebpf-syscall collector can run here.")
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
        return fail("ebpf-syscall collector requires linux", "Run this check inside WSL2 Ubuntu or another Linux host.")
    ok(runtime_label())
    return 0


def check_bpftrace() -> int:
    path = shutil.which("bpftrace")
    if path is None:
        return fail("bpftrace command not found", "Install it with: sudo apt-get install bpftrace")
    ok(f"bpftrace command found at {path}")
    return 0


def check_tracefs() -> int:
    tracefs_candidates = [Path("/sys/kernel/tracing"), Path("/sys/kernel/debug/tracing")]
    permission_errors: list[str] = []
    for path in tracefs_candidates:
        try:
            if (path / "available_events").exists():
                ok("tracefs is mounted and exposes available_events")
                return 0
        except PermissionError:
            permission_errors.append(str(path))
        except OSError as exc:
            permission_errors.append(f"{path} ({exc})")

    if permission_errors:
        return fail(
            "tracefs available_events is not readable",
            "Run inside WSL2 Ubuntu with tracing permissions or as root; if tracefs is mounted but unreadable, "
            "the current user cannot inspect /sys/kernel/tracing/available_events.",
        )
    return fail(
        "tracefs available_events not found",
        "Mount tracefs/debugfs or run inside a Linux host with tracing support enabled.",
    )


def check_pid(pid: int) -> int:
    if pid <= 0:
        return 0
    if not (Path("/proc") / str(pid)).exists():
        return fail(f"target pid {pid} not found", "Start scripts/demo/mock_target.py and use its printed target_pid.")
    ok(f"target pid {pid} exists")
    return 0


def check_bpftrace_smoke(pid: int) -> int:
    if pid <= 0 or shutil.which("bpftrace") is None:
        return 0

    program = f"tracepoint:syscalls:sys_enter_* /pid == {pid}/ {{ @[probe] = count(); }} interval:s:1 {{ exit(); }}"
    try:
        completed = subprocess.run(
            ["bpftrace", "-e", program],
            check=False,
            capture_output=True,
            text=True,
            timeout=8,
        )
    except subprocess.TimeoutExpired:
        return fail("bpftrace smoke timed out")

    if completed.returncode != 0:
        details = (completed.stderr or completed.stdout).strip()
        return fail(f"bpftrace smoke failed: {details}")

    output = completed.stdout.strip()
    if not output:
        return fail("bpftrace smoke produced no syscall samples", "Use an active target process for the smoke check.")

    ok("bpftrace syscall smoke command completed")
    return 0


def main() -> int:
    args = parse_args()
    checks = [
        check_linux(),
        check_bpftrace(),
        check_tracefs(),
        check_pid(args.pid),
        check_bpftrace_smoke(args.pid),
    ]
    if any(code != 0 for code in checks):
        print_next_steps(
            "ebpf-syscall collector",
            [
                "sudo apt-get update && sudo apt-get install -y bpftrace",
                "sudo mount -t tracefs tracefs /sys/kernel/tracing 2>/dev/null || true",
                "COLLECTOR_TYPE=ebpf-syscall bash ./scripts/demo/start-local.sh",
                "make smoke-real COLLECTOR_TYPE=ebpf-syscall",
            ],
        )
        return 1

    print("eBPF syscall collector prerequisites look ready.")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
