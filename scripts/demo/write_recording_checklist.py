import argparse
import os
from datetime import datetime, timezone
from pathlib import Path


ROOT = Path(__file__).resolve().parents[2]
DEFAULT_API_PORT = os.environ.get("MINIDROP_API_PORT", "8080")
DEFAULT_WEB_PORT = os.environ.get("MINIDROP_WEB_PORT", "4173")
DEFAULT_MINIO_PORT = os.environ.get("MINIDROP_MINIO_PORT", "9000")
DEFAULT_MINIO_CONSOLE_PORT = os.environ.get("MINIDROP_MINIO_CONSOLE_PORT", "9001")
DEFAULT_EVIDENCE_PATH = os.environ.get("MINIDROP_DEMO_EVIDENCE_OUTPUT", "artifacts/demo-evidence.md")
DEFAULT_ATTRIBUTION_EVALUATION_PATH = os.environ.get(
    "MINIDROP_ATTRIBUTION_EVALUATION_OUTPUT",
    "artifacts/attribution-evaluation-report.md",
)


def checklist_lines(
    api_port: str,
    web_port: str,
    minio_port: str,
    minio_console_port: str,
    evidence_path: str,
    attribution_evaluation_path: str,
) -> list[str]:
    generated_at = datetime.now(timezone.utc).strftime("%Y-%m-%d %H:%M:%S UTC")
    web_url = f"http://localhost:{web_port}"
    api_health_url = f"http://localhost:{api_port}/healthz"
    minio_url = f"http://localhost:{minio_console_port}"

    return [
        "# Mini-Drop Recording Checklist",
        "",
        f"Generated: {generated_at}",
        "",
        "## Open Before Recording",
        "",
        f"- Web console: `{web_url}`",
        f"- API health: `{api_health_url}`",
        f"- MinIO console: `{minio_url}`",
        f"- Evidence file: `{evidence_path}`",
        f"- Attribution evaluation: `{attribution_evaluation_path}`",
        "",
        "## Preflight Commands",
        "",
        "```powershell",
        f".\\scripts\\demo\\acceptance-snapshot.ps1 -ApiPort {api_port} -WebPort {web_port} -MinioPort {minio_port} -SeedTasks",
        f".\\scripts\\demo\\write-demo-evidence.ps1 -ApiPort {api_port} -WebPort {web_port} -MinioPort {minio_port} -IncludeRealPreflight",
        "python scripts\\demo\\write_attribution_evaluation.py",
        f".\\scripts\\demo\\write-recording-checklist.ps1 -ApiPort {api_port} -WebPort {web_port} -MinioPort {minio_port} -MinioConsolePort {minio_console_port}",
        "```",
        "",
        "```bash",
        "python3 scripts/demo/write_attribution_evaluation.py",
        f"MINIDROP_API_PORT={api_port} MINIDROP_WEB_PORT={web_port} MINIDROP_MINIO_PORT={minio_port} MINIDROP_MINIO_CONSOLE_PORT={minio_console_port} bash ./scripts/demo/write-recording-checklist.sh",
        "```",
        "",
        "## Capture List",
        "",
        "- Dashboard: online Agent count, completed tasks, Tencent Cloud style console layout.",
        "- Machine list: Agent heartbeat and ONLINE status.",
        "- History task detail: status history, flamegraph, TopN, attribution evidence.",
        "- File analysis: raw artifact, flamegraph SVG, TopN JSON, signed artifact URL.",
        "- Task comparison: TopN delta and recurring hotspot aggregate.",
        "- Plan page: continuous profile, interval/cron policy, stagger offset, windows, trend labels, baseline drift.",
        "- Coverage report: required gates and observed Agent coverage.",
        "- Attribution evaluation report: six scored samples and criterion details.",
        "- Failure path: FAILED task reason and status history.",
        "- Evidence file: acceptance snapshot, continuous profile sample, real collector preflight.",
        "",
        "## Acceptance Lines To Show",
        "",
        "- `acceptance=OK` from `acceptance-snapshot`.",
        "- `coverage=OK` from `make coverage`.",
        "- `attribution_evaluation=OK` from `make attribution-evaluation`.",
        "- `continuous_profiles` and `continuous_profile_samples` with schedule policy.",
        "- `minio_signed_results` greater than zero for the Compose path.",
        "- Recent Git commits with explanatory messages.",
        "",
        "## Closing",
        "",
        "The mock product path is complete and repeatable. The real collector path is coded and documented, with Linux validation gated on host prerequisites.",
    ]


def main() -> int:
    parser = argparse.ArgumentParser(description="Write a recording checklist for the Mini-Drop final demo.")
    parser.add_argument(
        "--output",
        default=str(ROOT / "artifacts" / "recording-checklist.md"),
        help="Output Markdown path. Defaults to artifacts/recording-checklist.md.",
    )
    parser.add_argument("--api-port", default=DEFAULT_API_PORT, help=f"API port. Defaults to {DEFAULT_API_PORT}.")
    parser.add_argument("--web-port", default=DEFAULT_WEB_PORT, help=f"Web port. Defaults to {DEFAULT_WEB_PORT}.")
    parser.add_argument("--minio-port", default=DEFAULT_MINIO_PORT, help=f"MinIO public port. Defaults to {DEFAULT_MINIO_PORT}.")
    parser.add_argument(
        "--minio-console-port",
        default=DEFAULT_MINIO_CONSOLE_PORT,
        help=f"MinIO console port. Defaults to {DEFAULT_MINIO_CONSOLE_PORT}.",
    )
    parser.add_argument(
        "--evidence-path",
        default=DEFAULT_EVIDENCE_PATH,
        help=f"Evidence path to show. Defaults to {DEFAULT_EVIDENCE_PATH}.",
    )
    parser.add_argument(
        "--attribution-evaluation-path",
        default=DEFAULT_ATTRIBUTION_EVALUATION_PATH,
        help=f"Attribution evaluation report path to show. Defaults to {DEFAULT_ATTRIBUTION_EVALUATION_PATH}.",
    )
    args = parser.parse_args()

    output_path = Path(args.output)
    if not output_path.is_absolute():
        output_path = ROOT / output_path
    output_path.parent.mkdir(parents=True, exist_ok=True)
    output_path.write_text(
        "\n".join(
            checklist_lines(
                api_port=str(args.api_port),
                web_port=str(args.web_port),
                minio_port=str(args.minio_port),
                minio_console_port=str(args.minio_console_port),
                evidence_path=str(args.evidence_path),
                attribution_evaluation_path=str(args.attribution_evaluation_path),
            )
        )
        + "\n",
        encoding="utf-8",
    )
    print(f"Wrote recording checklist to {output_path}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
