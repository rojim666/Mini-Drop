import argparse
import os
from datetime import datetime, timezone
from pathlib import Path


ROOT = Path(__file__).resolve().parents[2]
DEFAULT_API_PORT = os.environ.get("MINIDROP_API_PORT", "8080")
DEFAULT_WEB_PORT = os.environ.get("MINIDROP_WEB_PORT", "4173")
DEFAULT_MINIO_CONSOLE_PORT = os.environ.get("MINIDROP_MINIO_CONSOLE_PORT", "9001")
DEFAULT_EVIDENCE_PATH = os.environ.get("MINIDROP_DEMO_EVIDENCE_OUTPUT", "artifacts/demo-evidence.md")
DEFAULT_CHECKLIST_PATH = os.environ.get("MINIDROP_RECORDING_CHECKLIST_OUTPUT", "artifacts/recording-checklist.md")


def submission_lines(api_port: str, web_port: str, minio_console_port: str, evidence_path: str, checklist_path: str) -> list[str]:
    generated_at = datetime.now(timezone.utc).strftime("%Y-%m-%d %H:%M:%S UTC")
    web_url = f"http://localhost:{web_port}"
    api_url = f"http://localhost:{api_port}/healthz"
    minio_url = f"http://localhost:{minio_console_port}"

    screenshot_rows = [
        ["01-dashboard.png", web_url, "Dashboard overview, online Agent, completed tasks, console styling"],
        ["02-machines.png", web_url, "Agent heartbeat and ONLINE state"],
        ["03-task-detail.png", web_url, "Task status history, flamegraph, TopN, attribution evidence"],
        ["04-files.png", web_url, "Raw artifact, flamegraph SVG, topn.json, signed artifact URL"],
        ["05-compare.png", web_url, "Task delta table and recurring hotspot aggregate"],
        ["06-schedule.png", web_url, "Continuous profile, interval/cron policy, stagger offset, windows, trends"],
        ["07-failure-audit.png", web_url, "FAILED task reason, status event, Agent audit log"],
        ["08-evidence.png", evidence_path, "Acceptance snapshot and real collector preflight"],
        ["09-coverage.png", "artifacts/coverage-report.md", "Required coverage gates and observed Agent coverage"],
        ["10-minio.png", minio_url, "Object bucket view when reviewers ask for storage proof"],
    ]

    lines = [
        "# Mini-Drop Submission Notes",
        "",
        f"Generated: {generated_at}",
        "",
        "## Runtime Links",
        "",
        f"- Web console: `{web_url}`",
        f"- API health: `{api_url}`",
        f"- MinIO console: `{minio_url}`",
        f"- Evidence: `{evidence_path}`",
        f"- Recording checklist: `{checklist_path}`",
        "",
        "## Screenshot Manifest",
        "",
        "| File | Source | Must Show |",
        "| --- | --- | --- |",
    ]
    lines.extend(f"| {name} | {source} | {must_show} |" for name, source, must_show in screenshot_rows)
    lines.extend(
        [
            "",
            "## Recording Acceptance",
            "",
            "- Compose mock path starts successfully and `acceptance=OK` is visible.",
            "- At least two completed TopN-backed tasks are available for comparison.",
            "- MinIO signed URLs include `X-Amz-Signature` on the Compose path.",
            "- Continuous profiling evidence includes schedule policy and sampled windows.",
            "- Failure path shows a clear `status_reason` and status history.",
            "- Coverage report shows required gates at or above 50%.",
            "- Real collector preflight is either READY or BLOCKED with concrete next commands.",
            "",
            "## Verification Commands",
            "",
            "```powershell",
            "go test ./apps/api-server ./apps/agent ./internal/...",
            "python -m unittest apps.analyzer.main_test",
            "npm --prefix apps\\web run build",
            "python scripts\\demo\\check_coverage.py",
            "python -m py_compile scripts\\demo\\acceptance_snapshot.py scripts\\demo\\write_demo_evidence.py scripts\\demo\\write_recording_checklist.py scripts\\demo\\write_submission_notes.py",
            "```",
            "",
            "## Commit Summary Template",
            "",
            "- Mock E2E path: Web -> API -> Agent -> Analyzer -> Web is repeatable.",
            "- Compose delivery path: PostgreSQL, MinIO signed URLs, smoke tasks, and evidence scripts are wired.",
            "- Real collectors: perf, eBPF syscall, and py-spy code paths are implemented with Linux preflight checks.",
            "- Continuous profiling: interval/cron/stagger scheduling, trend labels, baseline drift, and comparison aggregate are visible.",
            "- Attribution: deterministic tool evidence, baseline comparison, prompt, and trace are persisted and shown.",
            "- Remaining external validation: Linux/WSL2 host permissions for real collectors and final manual recording.",
        ]
    )
    return lines


def main() -> int:
    parser = argparse.ArgumentParser(description="Write submission notes for Mini-Drop final delivery.")
    parser.add_argument(
        "--output",
        default=str(ROOT / "artifacts" / "submission-notes.md"),
        help="Output Markdown path. Defaults to artifacts/submission-notes.md.",
    )
    parser.add_argument("--api-port", default=DEFAULT_API_PORT, help=f"API port. Defaults to {DEFAULT_API_PORT}.")
    parser.add_argument("--web-port", default=DEFAULT_WEB_PORT, help=f"Web port. Defaults to {DEFAULT_WEB_PORT}.")
    parser.add_argument(
        "--minio-console-port",
        default=DEFAULT_MINIO_CONSOLE_PORT,
        help=f"MinIO console port. Defaults to {DEFAULT_MINIO_CONSOLE_PORT}.",
    )
    parser.add_argument(
        "--evidence-path",
        default=DEFAULT_EVIDENCE_PATH,
        help=f"Evidence path. Defaults to {DEFAULT_EVIDENCE_PATH}.",
    )
    parser.add_argument(
        "--checklist-path",
        default=DEFAULT_CHECKLIST_PATH,
        help=f"Recording checklist path. Defaults to {DEFAULT_CHECKLIST_PATH}.",
    )
    args = parser.parse_args()

    output_path = Path(args.output)
    if not output_path.is_absolute():
        output_path = ROOT / output_path
    output_path.parent.mkdir(parents=True, exist_ok=True)
    output_path.write_text(
        "\n".join(
            submission_lines(
                api_port=str(args.api_port),
                web_port=str(args.web_port),
                minio_console_port=str(args.minio_console_port),
                evidence_path=str(args.evidence_path),
                checklist_path=str(args.checklist_path),
            )
        )
        + "\n",
        encoding="utf-8",
    )
    print(f"Wrote submission notes to {output_path}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
