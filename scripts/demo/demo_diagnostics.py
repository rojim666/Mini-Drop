from __future__ import annotations

from urllib.parse import urlparse


def signed_url_ok(url: str, minio_port: str) -> bool:
    return f"localhost:{minio_port}" in url and "X-Amz-Signature=" in url


def minio_signed_url_failure_hints(
    agents: list[dict],
    result_urls: list[str],
    minio_port: str,
    api_base: str,
    web_port: str,
) -> list[str]:
    hints: list[str] = []
    agent_ids = sorted({str(agent.get("id") or "") for agent in agents if agent.get("id")})
    online_agent_ids = sorted(
        {str(agent.get("id") or "") for agent in agents if agent.get("id") and agent.get("status") == "ONLINE"}
    )

    if online_agent_ids and "agt_compose" not in online_agent_ids:
        hints.append(
            "Observed ONLINE agents "
            + ", ".join(online_agent_ids)
            + "; compose acceptance expects agt_compose, so this API looks like a local demo."
        )
    elif agent_ids and "agt_compose" not in agent_ids:
        hints.append(
            "Observed agents "
            + ", ".join(agent_ids)
            + "; compose acceptance expects agt_compose after start-compose."
        )

    local_artifact_urls = [url for url in result_urls if "/artifacts/" in url and "X-Amz-Signature=" not in url]
    if local_artifact_urls:
        parsed = urlparse(local_artifact_urls[0])
        host = parsed.netloc or "the API host"
        hints.append(
            f"DONE task results are local /artifacts URLs on {host}; compose acceptance needs MinIO signed URLs on localhost:{minio_port}."
        )

    signed_wrong_port = [
        url for url in result_urls if "X-Amz-Signature=" in url and f"localhost:{minio_port}" not in url
    ]
    if signed_wrong_port:
        parsed = urlparse(signed_wrong_port[0])
        hints.append(
            f"Signed URLs are present on {parsed.netloc or 'another host'}, but the check is using MinIO port {minio_port}."
        )

    hints.append(
        "For the compose path, start with "
        f".\\scripts\\demo\\start-compose.ps1 -ApiPort {urlparse(api_base).port or 8080} "
        f"-WebPort {web_port} -MinioPort {minio_port} and pass the same ports to final-preflight."
    )
    return hints


def missing_endpoint_hint(path: str, api_base: str) -> str:
    port = urlparse(api_base).port or 8080
    return (
        f"Endpoint {path} is not available on {api_base}; this usually means port {port} is serving an older/local API. "
        "Stop the local demo or pass the compose API port used by start-compose/final-preflight."
    )
