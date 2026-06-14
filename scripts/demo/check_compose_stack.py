import argparse
import json
import subprocess
import sys
from pathlib import Path
from typing import Any


ROOT = Path(__file__).resolve().parents[2]
DEFAULT_COMPOSE_FILE = ROOT / "deploy" / "docker-compose.yml"

REQUIRED_SERVICES = {
    "postgres": {"health": "healthy"},
    "minio": {"health": "healthy", "ports": {9000: "minio_port", 9001: "minio_console_port"}},
    "demo-target": {},
    "api-server": {"health": "healthy", "ports": {8080: "api_port"}},
    "agent": {},
    "web": {"health": "healthy", "ports": {80: "web_port"}},
}


def parse_compose_json(raw: str) -> list[dict[str, Any]]:
    value = raw.strip()
    if not value:
        return []
    try:
        parsed = json.loads(value)
        if isinstance(parsed, list):
            return [item for item in parsed if isinstance(item, dict)]
        if isinstance(parsed, dict):
            return [parsed]
    except json.JSONDecodeError:
        pass

    rows: list[dict[str, Any]] = []
    for line in value.splitlines():
        line = line.strip()
        if not line:
            continue
        item = json.loads(line)
        if isinstance(item, dict):
            rows.append(item)
    return rows


def published_port(service: dict[str, Any], target_port: int) -> int | None:
    publishers = service.get("Publishers") or []
    if not isinstance(publishers, list):
        return None
    for publisher in publishers:
        if not isinstance(publisher, dict):
            continue
        if int(publisher.get("TargetPort") or 0) == target_port:
            published = int(publisher.get("PublishedPort") or 0)
            if published > 0:
                return published
    return None


def check_services(services: list[dict[str, Any]], expected_ports: dict[str, int]) -> tuple[list[str], list[str]]:
    failures: list[str] = []
    lines: list[str] = []
    by_service = {str(service.get("Service") or ""): service for service in services}

    for name, requirements in REQUIRED_SERVICES.items():
        service = by_service.get(name)
        if service is None:
            failures.append(f"service {name} is missing from docker compose ps")
            lines.append(f"{name}: missing")
            continue

        state = str(service.get("State") or "")
        health = str(service.get("Health") or "")
        status = str(service.get("Status") or "")
        lines.append(f"{name}: state={state or '-'} health={health or '-'} status={status or '-'}")

        if state != "running":
            failures.append(f"service {name} expected running, got {state or '-'}")

        expected_health = requirements.get("health")
        if expected_health and health != expected_health:
            failures.append(f"service {name} expected health {expected_health}, got {health or '-'}")

        for target_port, port_key in requirements.get("ports", {}).items():
            expected_port = expected_ports[port_key]
            got_port = published_port(service, target_port)
            lines.append(f"{name}: port {got_port or '-'}->{target_port}, expected {expected_port}->{target_port}")
            if got_port != expected_port:
                failures.append(
                    f"service {name} expected localhost:{expected_port}->{target_port}, got {got_port or '-'}->{target_port}"
                )

    return failures, lines


def run_compose_ps(compose_file: Path) -> tuple[int, str]:
    try:
        result = subprocess.run(
            ["docker", "compose", "-f", str(compose_file), "ps", "--format", "json"],
            cwd=ROOT,
            check=False,
            capture_output=True,
            text=True,
            encoding="utf-8",
            errors="replace",
            timeout=30,
        )
    except FileNotFoundError as exc:
        return 127, str(exc)
    except subprocess.TimeoutExpired as exc:
        output = (exc.stdout or "") + (exc.stderr or "")
        return 124, output.strip() or "docker compose ps timed out"

    output = "\n".join(part for part in [result.stdout.strip(), result.stderr.strip()] if part)
    return result.returncode, output


def main() -> int:
    parser = argparse.ArgumentParser(description="Verify the Mini-Drop Docker Compose demo stack is running.")
    parser.add_argument("--compose-file", default=str(DEFAULT_COMPOSE_FILE), help="Path to deploy/docker-compose.yml.")
    parser.add_argument("--api-port", type=int, default=8080)
    parser.add_argument("--web-port", type=int, default=4173)
    parser.add_argument("--minio-port", type=int, default=9000)
    parser.add_argument("--minio-console-port", type=int, default=9001)
    args = parser.parse_args()

    compose_file = Path(args.compose_file)
    if not compose_file.is_absolute():
        compose_file = ROOT / compose_file

    code, output = run_compose_ps(compose_file)
    print("Mini-Drop compose stack check")
    print(f"compose_file={compose_file}")
    if code != 0:
        print(output or "(no output)")
        print("compose_stack=FAILED")
        return code

    try:
        services = parse_compose_json(output)
    except json.JSONDecodeError as exc:
        print(f"failed to parse docker compose ps JSON: {exc}")
        print(output)
        print("compose_stack=FAILED")
        return 1

    failures, lines = check_services(
        services,
        {
            "api_port": args.api_port,
            "web_port": args.web_port,
            "minio_port": args.minio_port,
            "minio_console_port": args.minio_console_port,
        },
    )
    for line in lines:
        print(line)

    if failures:
        print("compose_stack=FAILED")
        for failure in failures:
            print(f"- {failure}", file=sys.stderr)
        return 1

    print("compose_stack=OK")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
