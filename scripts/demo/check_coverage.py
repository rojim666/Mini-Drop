import argparse
import os
import re
import subprocess
import sys
from dataclasses import dataclass
from datetime import datetime, timezone
from pathlib import Path


ROOT = Path(__file__).resolve().parents[2]
DEFAULT_OUTPUT = ROOT / "artifacts" / "coverage-report.md"
COVERAGE_THRESHOLD = 50.0


@dataclass
class CoverageResult:
    name: str
    command: list[str]
    percent: float
    required: bool
    code: int
    output: str

    @property
    def ok(self) -> bool:
        return self.code == 0 and (not self.required or self.percent >= COVERAGE_THRESHOLD)


def executable(name: str) -> str:
    if os.name == "nt" and name in {"npm", "npx"}:
        return f"{name}.cmd"
    return name


def run_command(args: list[str], env: dict[str, str] | None = None, timeout: int = 180) -> tuple[int, str]:
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
    except FileNotFoundError as exc:
        return 127, str(exc)
    except subprocess.TimeoutExpired as exc:
        output = "\n".join(part for part in [exc.stdout or "", exc.stderr or ""] if part)
        return 124, output.strip() or f"{' '.join(args)} timed out after {timeout}s"

    output = "\n".join(part for part in [completed.stdout.strip(), completed.stderr.strip()] if part)
    return completed.returncode, output


def parse_go_total(output: str) -> float:
    match = re.search(r"^total:\s+\(statements\)\s+([0-9.]+)%", output, re.MULTILINE)
    if not match:
        raise ValueError("could not parse go coverage total")
    return float(match.group(1))


def parse_python_total(output: str) -> float:
    match = re.search(r"^TOTAL\s+\d+\s+\d+\s+([0-9.]+)%", output, re.MULTILINE)
    if not match:
        raise ValueError("could not parse python coverage total")
    return float(match.group(1))


def command_text(args: list[str]) -> str:
    return " ".join(args)


def run_go_coverage(name: str, package: str, profile_name: str, required: bool) -> CoverageResult:
    profile_path = ROOT / "tmp" / profile_name
    profile_path.parent.mkdir(parents=True, exist_ok=True)
    if profile_path.exists():
        profile_path.unlink()

    env = os.environ.copy()
    env["GOPROXY"] = env.get("GOPROXY", "https://goproxy.cn,direct")
    test_cmd = ["go", "test", f"-coverprofile=tmp/{profile_name}", package]
    test_code, test_output = run_command(test_cmd, env=env)
    cover_cmd = ["go", "tool", "cover", "-func", f"tmp/{profile_name}"]
    cover_code, cover_output = run_command(cover_cmd, env=env)
    combined = "\n".join(part for part in [test_output, cover_output] if part)
    code = test_code if test_code != 0 else cover_code

    percent = 0.0
    if code == 0:
        try:
            percent = parse_go_total(cover_output)
        except ValueError as exc:
            code = 1
            combined = f"{combined}\n{exc}".strip()

    return CoverageResult(name=name, command=test_cmd + ["&&", *cover_cmd], percent=percent, required=required, code=code, output=combined)


def run_python_coverage() -> CoverageResult:
    erase_cmd = [sys.executable, "-m", "coverage", "erase"]
    run_cmd = [sys.executable, "-m", "coverage", "run", "--source=apps/analyzer", "-m", "unittest", "apps.analyzer.main_test"]
    report_cmd = [sys.executable, "-m", "coverage", "report", "-m"]

    outputs: list[str] = []
    for cmd in [erase_cmd, run_cmd, report_cmd]:
        code, output = run_command(cmd)
        outputs.append(output)
        if code != 0:
            return CoverageResult("Python analyzer", run_cmd + ["&&", *report_cmd], 0.0, True, code, "\n".join(outputs))

    report_output = outputs[-1]
    try:
        percent = parse_python_total(report_output)
    except ValueError as exc:
        return CoverageResult("Python analyzer", run_cmd + ["&&", *report_cmd], 0.0, True, 1, f"{report_output}\n{exc}")

    return CoverageResult("Python analyzer", run_cmd + ["&&", *report_cmd], percent, True, 0, "\n".join(outputs))


def markdown_table(headers: list[str], rows: list[list[str]]) -> list[str]:
    lines = [
        "| " + " | ".join(headers) + " |",
        "| " + " | ".join("---" for _ in headers) + " |",
    ]
    for row in rows:
        lines.append("| " + " | ".join(cell.replace("|", "\\|") for cell in row) + " |")
    return lines


def write_report(output_path: Path, results: list[CoverageResult]) -> None:
    generated_at = datetime.now(timezone.utc).strftime("%Y-%m-%d %H:%M:%S UTC")
    failed = [result for result in results if result.required and not result.ok]
    rows = [
        [
            result.name,
            "yes" if result.required else "no",
            f"{result.percent:.1f}%",
            "OK" if result.ok else f"FAILED ({result.code})",
        ]
        for result in results
    ]

    lines = [
        "# Mini-Drop Coverage Report",
        "",
        f"Generated: {generated_at}",
        "",
        f"Required threshold: `{COVERAGE_THRESHOLD:.0f}%` for required gates.",
        "",
        "## Summary",
        "",
    ]
    lines.extend(markdown_table(["Area", "Required", "Coverage", "Status"], rows))
    lines.extend(
        [
            "",
            "## Scope",
            "",
            "- Required gates cover the API orchestration layer, shared validation/status contracts, and Analyzer transformation logic.",
            "- Agent coverage is reported as observational because real collectors involve OS tools and host permissions; Agent behavior is still covered by integration smoke tests and focused unit tests.",
            "",
            "## Output",
            "",
        ]
    )

    for result in results:
        lines.extend(
            [
                f"### {result.name}",
                "",
                f"- Command: `{command_text(result.command)}`",
                f"- Required: `{'yes' if result.required else 'no'}`",
                f"- Coverage: `{result.percent:.1f}%`",
                f"- Exit code: `{result.code}`",
                "",
                "```text",
                result.output or "(no output)",
                "```",
                "",
            ]
        )

    if failed:
        lines.extend(["## Failures", ""])
        lines.extend(f"- `{result.name}` is below {COVERAGE_THRESHOLD:.0f}% or failed to run." for result in failed)

    output_path.parent.mkdir(parents=True, exist_ok=True)
    output_path.write_text("\n".join(lines) + "\n", encoding="utf-8")


def main() -> int:
    parser = argparse.ArgumentParser(description="Run Mini-Drop coverage gates and write a Markdown report.")
    parser.add_argument("--output", default=str(DEFAULT_OUTPUT), help="Output Markdown path.")
    args = parser.parse_args()

    results = [
        run_go_coverage("Go API orchestration", "./internal/apiserver", "apicover", required=True),
        run_go_coverage("Go shared contracts", "./internal/minidrop", "minidropcover", required=True),
        run_go_coverage("Go Agent", "./internal/agent", "agentcover", required=False),
        run_python_coverage(),
    ]

    output_path = Path(args.output)
    if not output_path.is_absolute():
        output_path = ROOT / output_path
    write_report(output_path, results)

    failed = [result for result in results if result.required and not result.ok]
    print(f"Wrote coverage report to {output_path}")
    for result in results:
        print(f"{result.name}: {result.percent:.1f}% ({'required' if result.required else 'observed'})")
    if failed:
        print("coverage=FAILED")
        return 1
    print("coverage=OK")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
