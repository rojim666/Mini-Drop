import argparse
import subprocess
import sys
from pathlib import Path


ROOT = Path(__file__).resolve().parents[2]
SCRIPT_DIR = ROOT / "scripts" / "demo"


CHECKS = {
    "perf": "check_perf_env.py",
    "ebpf-syscall": "check_ebpf_env.py",
    "py-spy": "check_pyspy_env.py",
}


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Run all Mini-Drop real collector preflight checks.")
    parser.add_argument("--pid", type=int, default=0, help="Optional target PID to validate.")
    parser.add_argument(
        "--collectors",
        default="perf,ebpf-syscall,py-spy",
        help="Comma-separated collectors to check. Defaults to perf,ebpf-syscall,py-spy.",
    )
    return parser.parse_args()


def run_check(name: str, pid: int) -> int:
    script = CHECKS[name]
    cmd = [sys.executable, str(SCRIPT_DIR / script)]
    if pid > 0:
        cmd.extend(["--pid", str(pid)])

    print(f"\n=== {name} preflight ===", flush=True)
    completed = subprocess.run(cmd, check=False)
    if completed.returncode == 0:
        print(f"READY: {name}")
    else:
        print(f"BLOCKED: {name}")
    return completed.returncode


def main() -> int:
    args = parse_args()
    requested = [item.strip() for item in args.collectors.split(",") if item.strip()]
    unknown = [item for item in requested if item not in CHECKS]
    if unknown:
        print(f"Unsupported collectors: {', '.join(unknown)}", file=sys.stderr)
        print(f"Supported collectors: {', '.join(CHECKS)}", file=sys.stderr)
        return 2

    results: dict[str, int] = {}
    for collector in requested:
        results[collector] = run_check(collector, args.pid)

    ready = [name for name, code in results.items() if code == 0]
    blocked = [name for name, code in results.items() if code != 0]

    print("\n=== Summary ===")
    print("ready: " + (", ".join(ready) if ready else "none"))
    print("blocked: " + (", ".join(blocked) if blocked else "none"))

    if blocked:
        print("\nRun the blocked collector section above and apply its listed next steps.")
        return 1

    print("\nAll requested real collector prerequisites look ready.")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
