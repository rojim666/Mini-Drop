import argparse
import json
import os
import subprocess
import sys
import time
from dataclasses import dataclass
from datetime import datetime, timezone
from pathlib import Path
from typing import Any


ROOT = Path(__file__).resolve().parents[2]
DEFAULT_OUTPUT = ROOT / "artifacts" / "attribution-evaluation-report.md"
EVAL_PREFIX = "ATTRIBUTION_EVAL "


@dataclass
class EvaluationResult:
    id: str
    scenario: str
    collector_type: str
    expected: str
    top_function: str
    conclusion: str
    confidence: float
    timeline_source: str
    timeline_signal: str
    evidence_kinds: list[str]
    tool_names: list[str]
    score: int
    max_score: int
    passed: bool
    criteria: list[dict[str, Any]]

    @property
    def score_percent(self) -> float:
        if self.max_score <= 0:
            return 0.0
        return round((self.score / self.max_score) * 100, 1)


@dataclass
class TestRun:
    code: int
    command: list[str]
    output: str
    elapsed_sec: float
    results: list[EvaluationResult]

    @property
    def passed(self) -> bool:
        return self.code == 0 and bool(self.results) and all(result.passed for result in self.results)


def quote_command(args: list[str]) -> str:
    return " ".join(f'"{arg}"' if any(char.isspace() for char in arg) else arg for arg in args)


def run_evaluation(timeout: int) -> TestRun:
    command = ["go", "test", "-run", "TestAttributionEvaluationSamples", "-v", "./internal/apiserver"]
    env = os.environ.copy()
    env["GOPROXY"] = env.get("GOPROXY", "https://goproxy.cn,direct")
    started = time.monotonic()
    try:
        completed = subprocess.run(
            command,
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
        output = "\n".join(part.strip() for part in [exc.stdout or "", exc.stderr or ""] if part)
        if not output:
            output = f"{quote_command(command)} timed out after {timeout}s"
        code = 124

    return TestRun(
        code=code,
        command=command,
        output=output or "(no output)",
        elapsed_sec=time.monotonic() - started,
        results=parse_results(output),
    )


def parse_results(output: str) -> list[EvaluationResult]:
    results: list[EvaluationResult] = []
    for line in output.splitlines():
        marker = line.find(EVAL_PREFIX)
        if marker < 0:
            continue
        payload = line[marker + len(EVAL_PREFIX) :].strip()
        if not payload:
            continue
        data = json.loads(payload)
        results.append(
            EvaluationResult(
                id=str(data.get("id") or ""),
                scenario=str(data.get("scenario") or ""),
                collector_type=str(data.get("collector_type") or ""),
                expected=str(data.get("expected") or ""),
                top_function=str(data.get("top_function") or ""),
                conclusion=str(data.get("conclusion") or ""),
                confidence=float(data.get("confidence") or 0.0),
                timeline_source=str(data.get("timeline_source") or ""),
                timeline_signal=str(data.get("timeline_signal") or ""),
                evidence_kinds=[str(item) for item in data.get("evidence_kinds") or []],
                tool_names=[str(item) for item in data.get("tool_names") or []],
                score=int(data.get("score") or 0),
                max_score=int(data.get("max_score") or 0),
                passed=bool(data.get("passed")),
                criteria=list(data.get("criteria") or []),
            )
        )
    return results


def markdown_cell(value: object) -> str:
    return str(value).replace("|", "\\|")


def markdown_table(headers: list[str], rows: list[list[str]]) -> list[str]:
    lines = [
        "| " + " | ".join(headers) + " |",
        "| " + " | ".join("---" for _ in headers) + " |",
    ]
    for row in rows:
        lines.append("| " + " | ".join(markdown_cell(cell) for cell in row) + " |")
    return lines


def truncate(output: str, limit: int = 7000) -> str:
    if len(output) <= limit:
        return output
    head = output[: limit // 2].rstrip()
    tail = output[-limit // 2 :].lstrip()
    return f"{head}\n\n... output truncated ...\n\n{tail}"


def write_report(output_path: Path, run: TestRun) -> None:
    generated_at = datetime.now(timezone.utc).strftime("%Y-%m-%d %H:%M:%S UTC")
    sample_count = len(run.results)
    pass_count = sum(1 for result in run.results if result.passed)
    total_score = sum(result.score for result in run.results)
    max_score = sum(result.max_score for result in run.results)
    score_percent = round((total_score / max_score) * 100, 1) if max_score else 0.0
    overall = "OK" if run.passed else "FAILED"
    rows = [
        [
            result.id,
            result.collector_type,
            result.expected,
            f"{result.score}/{result.max_score} ({result.score_percent:.1f}%)",
            "OK" if result.passed else "FAILED",
        ]
        for result in run.results
    ]

    lines = [
        "# Mini-Drop Attribution Evaluation Report",
        "",
        f"Generated: {generated_at}",
        "",
        "## Overall",
        "",
        f"- Status: `{overall}`",
        f"- Samples: `{pass_count}/{sample_count}` passed",
        f"- Score: `{total_score}/{max_score}` (`{score_percent:.1f}%`)",
        f"- Command: `{quote_command(run.command)}`",
        f"- Exit code: `{run.code}`",
        f"- Elapsed: `{run.elapsed_sec:.1f}s`",
        "",
        "## Scoring Rubric",
        "",
        "| Criterion | Weight | What It Proves |",
        "| --- | --- | --- |",
        "| conclusion | 2 | The generated conclusion matches the expected attribution class. |",
        "| evidence | 2 | Required evidence kinds are present and reviewable. |",
        "| timeline | 2 | Resource timeline source, signal, and points are attached. |",
        "| tool_trace | 2 | The deterministic tool loop stayed auditable. |",
        "| recommendation | 1 | Recommendations include the expected remediation direction. |",
        "| confidence | 1 | Confidence stays inside the expected range for that sample. |",
        "",
        "## Sample Summary",
        "",
    ]
    lines.extend(markdown_table(["Sample", "Collector", "Expected", "Score", "Status"], rows))

    lines.extend(["", "## Sample Details", ""])
    for result in run.results:
        lines.extend(
            [
                f"### {result.id}",
                "",
                f"- Scenario: {result.scenario}",
                f"- Collector: `{result.collector_type}`",
                f"- Top function: `{result.top_function}`",
                f"- Conclusion: {result.conclusion}",
                f"- Confidence: `{result.confidence:.2f}`",
                f"- Timeline: `{result.timeline_source}` / `{result.timeline_signal}`",
                f"- Evidence kinds: `{', '.join(result.evidence_kinds)}`",
                f"- Tool trace: `{', '.join(result.tool_names)}`",
                "",
                "| Criterion | Weight | Status | Detail |",
                "| --- | --- | --- | --- |",
            ]
        )
        for criterion in result.criteria:
            lines.append(
                "| "
                + " | ".join(
                    [
                        markdown_cell(criterion.get("name") or "-"),
                        markdown_cell(criterion.get("weight") or "-"),
                        "OK" if criterion.get("passed") else "FAILED",
                        markdown_cell(criterion.get("detail") or "-"),
                    ]
                )
                + " |"
            )
        lines.append("")

    lines.extend(
        [
            "## Raw Test Output",
            "",
            "```text",
            truncate(run.output),
            "```",
            "",
        ]
    )

    output_path.parent.mkdir(parents=True, exist_ok=True)
    output_path.write_text("\n".join(lines), encoding="utf-8")


def main() -> int:
    parser = argparse.ArgumentParser(description="Write the Mini-Drop attribution evaluation report.")
    parser.add_argument("--output", default=str(DEFAULT_OUTPUT), help="Output Markdown path.")
    parser.add_argument("--timeout", type=int, default=120, help="go test timeout in seconds.")
    args = parser.parse_args()

    output_path = Path(args.output)
    if not output_path.is_absolute():
        output_path = ROOT / output_path

    try:
        run = run_evaluation(args.timeout)
    except json.JSONDecodeError as exc:
        print(f"failed to parse attribution evaluation output: {exc}", file=sys.stderr)
        return 1

    write_report(output_path, run)
    print(f"Wrote attribution evaluation report to {output_path}")
    print(f"attribution_evaluation={'OK' if run.passed else 'FAILED'}")
    return 0 if run.passed else 1


if __name__ == "__main__":
    raise SystemExit(main())
