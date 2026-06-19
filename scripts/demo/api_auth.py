from __future__ import annotations

import json
import os
import urllib.request


def auth_headers(api_base: str) -> dict[str, str]:
    token = os.environ.get("MINIDROP_AUTH_TOKEN", "").strip()
    if not token:
        token = login_token(api_base)
        os.environ["MINIDROP_AUTH_TOKEN"] = token
    return {"Authorization": f"Bearer {token}"}


def login_token(api_base: str) -> str:
    username = os.environ.get("MINIDROP_AUTH_USERNAME", "demo")
    password = os.environ.get("MINIDROP_AUTH_PASSWORD", "minidrop")
    body = {
        "username": username,
        "password": password,
        "tenant": os.environ.get("MINIDROP_AUTH_TENANT", "local-demo"),
        "region": os.environ.get("MINIDROP_AUTH_REGION", "local"),
    }
    data = json.dumps(body).encode("utf-8")
    req = urllib.request.Request(
        f"{api_base.rstrip('/')}/api/v1/auth/login",
        method="POST",
        data=data,
        headers={"Content-Type": "application/json"},
    )
    with urllib.request.urlopen(req, timeout=10) as response:
        payload = json.loads(response.read().decode("utf-8"))
    token = str(payload.get("token") or "")
    if not token:
        raise RuntimeError("login response did not include token")
    return token
