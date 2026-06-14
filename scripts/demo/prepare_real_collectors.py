import argparse
import platform
import shutil
import subprocess
import sys
from datetime import datetime, timezone
from pathlib import Path


ROOT = Path(__file__).resolve().parents[2]
DEFAULT_OUTPUT = ROOT / "artifacts" / "real-collector-preflight.md"


def run_command(args: list[str], timeout: int = 120) -> tuple[int, str]:
    try:
        completed = subprocess.run(
            args,
            cwd=ROOT,
            check=False,
            capture_output=True,
            text=True,
            encoding="utf-8",
            errors="replace",
            timeout=timeout,
        )
    except FileNotFoundError as exc:
        return 127, str(exc)
    except subprocess.TimeoutExpired as exc:
        output = "\n".join(part for part in [exc.stdout or "", exc.stderr or ""] if part)
        return 124, output.strip() or f"{' '.join(args)} timed out after {timeout}s"

    output = "\n".join(part for part in [completed.stdout.strip(), completed.stderr.strip()] if part)
    return completed.returncode, output


def is_linux() -> bool:
    return platform.system().lower() == "linux"


def command_exists(name: str) -> bool:
    return shutil.which(name) is not None


def read_perf_paranoid() -> str:
    path = Path("/proc/sys/kernel/perf_event_paranoid")
    if not path.exists():
        return "missing"
    try:
        return path.read_text(encoding="utf-8").strip()
    except OSError as exc:
        return f"unreadable: {exc}"


def tracefs_state() -> str:
    candidates = [Path("/sys/kernel/tracing/available_events"), Path("/sys/kernel/debug/tracing/available_events")]
    states: list[str] = []
    for path in candidates:
        try:
            if path.exists():
                states.append(f"{path}: readable")
        except PermissionError:
            states.append(f"{path}: permission denied")
        except OSError as exc:
            states.append(f"{path}: {exc}")
    return ", ".join(states) if states else "available_events not found"


def install_commands() -> list[list[str]]:
    commands: list[list[str]] = []
    if command_exists("sudo"):
        sudo = ["sudo"]
    else:
        sudo = []

    if command_exists("apt-get"):
        commands.extend(
            [
                [*sudo, "apt-get", "update"],
                [*sudo, "apt-get", "install", "-y", "linux-tools-common", "linux-tools-generic", "bpftrace"],
            ]
        )

    if command_exists(sys.executable):
        commands.append([sys.executable, "-m", "pip", "install", "--user", "py-spy"])

    if command_exists("sysctl"):
        commands.append([*sudo, "sysctl", "kernel.perf_event_paranoid=1"])

    if command_exists("mount"):
        commands.append([*sudo, "mount", "-t", "tracefs", "tracefs", "/sys/kernel/tracing"])

    return commands


def run_install() -> list[tuple[list[str], int, str]]:
    results: list[tuple[list[str], int, str]] = []
    for command in install_commands():
        code, output = run_command(command, timeout=300)
        results.append((command, code, output))
    return results


def run_preflight(collectors: str, pid: int) -> tuple[int, str]:
    args = [sys.executable, str(ROOT / "scripts" / "demo" / "check_real_collectors.py"), "--collectors", collectors]
    if pid > 0:
        args.extend(["--pid", str(pid)])
    return run_command(args, timeout=180)


def command_text(args: list[str]) -> str:
    return " ".join(args)


def write_report(output_path: Path, collectors: str, pid: int, install_results: list[tuple[list[str], int, str]], preflight_code: int, preflight_output: str) -> None:
    generated_at = datetime.now(timezone.utc).strftime("%Y-%m-%d %H:%M:%S UTC")
    install_lines: list[str] = []
    if install_results:
        for command, code, output in install_results:
            install_lines.extend(
                [
                    f"### `{command_text(command)}`",
                    "",
                    f"- Exit code: `{code}`",
                    "",
                    "```text",
                    output or "(no output)",
                    "```",
                    "",
                ]
            )
    else:
        install_lines.append("_Install commands were not executed. Run with `--install` inside WSL2 / Linux after reviewing the commands._")

    suggested = [
        "sudo apt-get update",
        "sudo apt-get install -y linux-tools-common linux-tools-generic bpftrace",
        "python3 -m pip install --user py-spy",
        "sudo sysctl kernel.perf_event_paranoid=1",
        "sudo mount -t tracefs tracefs /sys/kernel/tracing 2>/dev/null || true",
        "COLLECTOR_TYPE=perf bash ./scripts/demo/start-local.sh",
        "make smoke-real COLLECTOR_TYPE=perf",
        "make smoke-real COLLECTOR_TYPE=ebpf-syscall",
        "make smoke-real COLLECTOR_TYPE=py-spy",
    ]

    lines = [
        "# Mini-Drop Real Collector Preflight",
        "",
        f"Generated: {generated_at}",
        "",
        "## Runtime",
        "",
        f"- Platform: `{platform.platform()}`",
        f"- Linux runtime: `{'yes' if is_linux() else 'no'}`",
        f"- Python: `{sys.version.split()[0]}`",
        f"- perf: `{shutil.which('perf') or 'missing'}`",
        f"- bpftrace: `{shutil.which('bpftrace') or 'missing'}`",
        f"- py-spy: `{shutil.which('py-spy') or 'missing'}`",
        f"- perf_event_paranoid: `{read_perf_paranoid()}`",
        f"- tracefs: `{tracefs_state()}`",
        f"- Target PID: `{pid if pid > 0 else 'not provided'}`",
        f"- Collectors: `{collectors}`",
        "",
        "## Suggested Commands",
        "",
        "```bash",
        "\n".join(suggested),
        "```",
        "",
        "## Install Output",
        "",
        *install_lines,
        "## Preflight Output",
        "",
        f"- Exit code: `{preflight_code}`",
        f"- Result: `{'READY' if preflight_code == 0 else 'BLOCKED'}`",
        "",
        "```text",
        preflight_output or "(no output)",
        "```",
    ]

    output_path.parent.mkdir(parents=True, exist_ok=True)
    output_path.write_text("\n".join(lines) + "\n", encoding="utf-8")


def main() -> int:
    parser = argparse.ArgumentParser(description="Prepare and document Mini-Drop real collector prerequisites.")
    parser.add_argument("--collectors", default="perf,ebpf-syscall,py-spy", help="Comma-separated collector list.")
    parser.add_argument("--pid", type=int, default=0, help="Optional target PID for smoke-capable checks.")
    parser.add_argument("--output", default=str(DEFAULT_OUTPUT), help="Markdown report path.")
    parser.add_argument("--install", action="store_true", help="Run install and permission commands before preflight.")
    args = parser.parse_args()

    if not is_linux():
        print("This helper is intended for WSL2 / Linux. Writing a blocked report from the current runtime.", file=sys.stderr)

    install_results = run_install() if args.install else []
    preflight_code, preflight_output = run_preflight(args.collectors, args.pid)

    output_path = Path(args.output)
    if not output_path.is_absolute():
        output_path = ROOT / output_path
    write_report(output_path, args.collectors, args.pid, install_results, preflight_code, preflight_output)

    print(f"Wrote real collector preflight report to {output_path}")
    return preflight_code


if __name__ == "__main__":
    raise SystemExit(main())
