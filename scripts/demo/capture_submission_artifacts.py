from __future__ import annotations

import argparse
import asyncio
import html
import os
from dataclasses import dataclass
from pathlib import Path
from typing import Iterable, Literal

from playwright.async_api import Error as PlaywrightError
from playwright.async_api import async_playwright


ROOT = Path(__file__).resolve().parents[2]
DEFAULT_WEB_PORT = "80"
DEFAULT_WEB_BASE = f"http://localhost:{DEFAULT_WEB_PORT}"
DEFAULT_MINIO_CONSOLE_PORT = "9001"
DEFAULT_MINIO_CONSOLE_BASE = f"http://localhost:{DEFAULT_MINIO_CONSOLE_PORT}"
DEFAULT_EVIDENCE_PATH = ROOT / "artifacts" / "demo-evidence.md"
DEFAULT_COVERAGE_PATH = ROOT / "artifacts" / "coverage-report.md"
DEFAULT_OUTPUT_DIR = ROOT / "artifacts" / "submission-screenshots"
DEFAULT_MINIO_USER = "minidrop"
DEFAULT_MINIO_PASSWORD = "minidrop123"
DEFAULT_WEB_USER = os.environ.get("MINIDROP_AUTH_USERNAME", "demo")
DEFAULT_WEB_PASSWORD = os.environ.get("MINIDROP_AUTH_PASSWORD", "minidrop")

CaptureKind = Literal["web", "markdown", "minio"]


@dataclass(frozen=True)
class CapturePage:
    name: str
    kind: CaptureKind
    path: str
    wait_for: str | None = None
    selector: str | None = None
    title: str | None = None


PAGES: list[CapturePage] = [
    CapturePage("01-dashboard.png", "web", "/#home", wait_for="text=Demo 证据链"),
    CapturePage("02-machines.png", "web", "/#machines", wait_for="text=Agent 状态"),
    CapturePage("03-task-detail.png", "web", "/#history", wait_for="text=历史任务"),
    CapturePage("04-files.png", "web", "/#files", wait_for="text=文件分析"),
    CapturePage("05-compare.png", "web", "/#compare", wait_for="text=任务对比"),
    CapturePage("06-schedule.png", "web", "/#schedule", wait_for="text=计划任务"),
    CapturePage("07-failure-audit.png", "web", "/#history", wait_for="text=target pid not found"),
    CapturePage("08-evidence.png", "markdown", "", title="Mini-Drop Demo Evidence"),
    CapturePage("09-coverage.png", "markdown", "", title="Mini-Drop Coverage Report"),
    CapturePage("10-minio.png", "minio", "/", wait_for="text=mini-drop-artifacts"),
]


async def wait_for_selector(page, selector: str) -> None:
    if selector.startswith("text="):
        await page.get_by_text(selector[5:], exact=False).first.wait_for(state="visible", timeout=15000)
    else:
        await page.locator(selector).first.wait_for(state="visible", timeout=15000)


async def login_to_web_if_needed(page, user: str, password: str) -> None:
    login_button = page.get_by_role("button", name="登录控制台").first
    try:
        await login_button.wait_for(state="visible", timeout=3000)
    except PlaywrightError:
        return

    try:
        await page.locator('input[type="text"]').first.fill(user, timeout=3000)
        await page.locator('input[type="password"]').first.fill(password, timeout=3000)
        await login_button.click(timeout=3000)
        await page.get_by_text("我的机器", exact=True).first.wait_for(state="visible", timeout=15000)
    except PlaywrightError as exc:
        raise RuntimeError(f"Web login failed before screenshot capture: {exc}") from exc


async def capture_one(page, base_url: str, output_dir: Path, capture: CapturePage) -> None:
    await page.goto(f"{base_url}{capture.path}", wait_until="domcontentloaded")
    await login_to_web_if_needed(page, DEFAULT_WEB_USER, DEFAULT_WEB_PASSWORD)
    if page.url.rstrip("/") == base_url.rstrip("/"):
        await page.goto(f"{base_url}{capture.path}", wait_until="domcontentloaded")
    if capture.wait_for:
        await wait_for_selector(page, capture.wait_for)
    if capture.selector:
        await wait_for_selector(page, capture.selector)
    if capture.name == "03-task-detail.png":
        done_row = page.locator("tr", has_text="已完成").first
        try:
            await done_row.click(timeout=5000)
            await page.get_by_text("火焰图", exact=True).wait_for(state="visible", timeout=5000)
            await page.get_by_text("热点 TopN", exact=True).wait_for(state="visible", timeout=5000)
        except PlaywrightError:
            pass
    elif capture.name == "07-failure-audit.png":
        failed_row = page.locator("tr", has_text="target pid not found").first
        try:
            await failed_row.click(timeout=5000)
            await page.get_by_text("状态历史", exact=True).wait_for(state="visible", timeout=5000)
        except PlaywrightError:
            pass
    await page.screenshot(path=str(output_dir / capture.name), full_page=True)


def render_markdown_document(path: Path, title: str) -> str:
    if not path.is_absolute():
        path = ROOT / path
    body = path.read_text(encoding="utf-8") if path.is_file() else f"{path} was not generated yet."
    escaped_body = html.escape(body)
    escaped_title = html.escape(title)
    return f"""<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <title>{escaped_title}</title>
  <style>
    :root {{
      color: #1f2733;
      background: #f4f6fb;
      font-family: Inter, "Segoe UI", Arial, sans-serif;
    }}
    body {{
      margin: 0;
      padding: 32px;
    }}
    main {{
      max-width: 1180px;
      margin: 0 auto;
      background: #fff;
      border: 1px solid #dfe5f2;
      border-radius: 6px;
      box-shadow: 0 8px 28px rgba(20, 45, 95, 0.08);
      overflow: hidden;
    }}
    header {{
      padding: 18px 24px;
      border-bottom: 1px solid #e7ecf5;
      background: linear-gradient(180deg, #fff, #f9fbff);
    }}
    h1 {{
      margin: 0;
      font-size: 22px;
      font-weight: 600;
    }}
    pre {{
      margin: 0;
      padding: 24px;
      white-space: pre-wrap;
      word-break: break-word;
      font: 13px/1.55 Consolas, "SFMono-Regular", monospace;
      color: #243040;
    }}
  </style>
</head>
<body>
  <main>
    <header><h1>{escaped_title}</h1></header>
    <pre>{escaped_body}</pre>
  </main>
</body>
</html>"""


async def capture_markdown(page, output_dir: Path, capture: CapturePage, markdown_path: Path) -> None:
    await page.set_content(render_markdown_document(markdown_path, capture.title or capture.name), wait_until="load")
    await page.screenshot(path=str(output_dir / capture.name), full_page=True)


async def login_to_minio_if_needed(page, user: str, password: str) -> None:
    user_inputs = [
        page.get_by_label("Access Key", exact=False),
        page.get_by_label("Username", exact=False),
        page.locator('input[name="accessKey"]'),
        page.locator('input[name="username"]'),
        page.locator('input[type="text"]').first,
    ]
    password_inputs = [
        page.get_by_label("Secret Key", exact=False),
        page.get_by_label("Password", exact=False),
        page.locator('input[name="secretKey"]'),
        page.locator('input[name="password"]'),
        page.locator('input[type="password"]').first,
    ]

    user_input = None
    for candidate in user_inputs:
        try:
            await candidate.wait_for(state="visible", timeout=2000)
            user_input = candidate
            break
        except PlaywrightError:
            continue
    if user_input is None:
        return

    password_input = None
    for candidate in password_inputs:
        try:
            await candidate.wait_for(state="visible", timeout=2000)
            password_input = candidate
            break
        except PlaywrightError:
            continue
    if password_input is None:
        return

    await user_input.fill(user)
    await password_input.fill(password)
    buttons = [
        page.get_by_role("button", name="Login").first,
        page.get_by_role("button", name="Sign in").first,
        page.locator('button[type="submit"]').first,
    ]
    for button in buttons:
        try:
            await button.click(timeout=3000)
            break
        except PlaywrightError:
            continue
    else:
        await password_input.press("Enter")


async def capture_minio(page, minio_base: str, output_dir: Path, capture: CapturePage, user: str, password: str) -> None:
    await page.goto(f"{minio_base.rstrip('/')}{capture.path}", wait_until="domcontentloaded")
    await login_to_minio_if_needed(page, user, password)
    try:
        await page.get_by_text("mini-drop-artifacts", exact=False).first.wait_for(state="visible", timeout=15000)
    except PlaywrightError:
        pass
    await page.screenshot(path=str(output_dir / capture.name), full_page=True)


async def launch_browser(playwright, channel: str):
    launch_attempts = []
    if channel != "auto":
        launch_attempts.append({"channel": channel})
    else:
        launch_attempts.extend([{"channel": "msedge"}, {"channel": "chrome"}, {}])

    errors: list[str] = []
    for kwargs in launch_attempts:
        try:
            return await playwright.chromium.launch(headless=True, **kwargs)
        except PlaywrightError as exc:
            label = kwargs.get("channel", "playwright-chromium")
            errors.append(f"{label}: {str(exc).splitlines()[0]}")

    raise RuntimeError(
        "Unable to launch a browser for screenshots. Install Microsoft Edge/Chrome or run `python -m playwright install chromium`.\n"
        + "\n".join(errors)
    )


async def main_async(
    output_dir: Path,
    base_url: str,
    minio_base: str,
    evidence_path: Path,
    coverage_path: Path,
    pages: Iterable[CapturePage],
    channel: str,
    minio_user: str,
    minio_password: str,
) -> None:
    output_dir.mkdir(parents=True, exist_ok=True)
    async with async_playwright() as playwright:
        browser = await launch_browser(playwright, channel)
        page = await browser.new_page(viewport={"width": 1440, "height": 1600}, device_scale_factor=1)
        try:
            for capture in pages:
                if capture.kind == "web":
                    await capture_one(page, base_url, output_dir, capture)
                elif capture.kind == "markdown":
                    markdown_path = evidence_path if capture.name == "08-evidence.png" else coverage_path
                    await capture_markdown(page, output_dir, capture, markdown_path)
                else:
                    await capture_minio(page, minio_base, output_dir, capture, minio_user, minio_password)
        finally:
            await browser.close()


def main() -> int:
    parser = argparse.ArgumentParser(description="Capture Mini-Drop submission screenshots.")
    parser.add_argument("--web-base", default=DEFAULT_WEB_BASE, help=f"Web base URL. Defaults to {DEFAULT_WEB_BASE}.")
    parser.add_argument(
        "--minio-console-base",
        default=DEFAULT_MINIO_CONSOLE_BASE,
        help=f"MinIO console URL. Defaults to {DEFAULT_MINIO_CONSOLE_BASE}.",
    )
    parser.add_argument(
        "--evidence-path",
        default=str(DEFAULT_EVIDENCE_PATH),
        help="Demo evidence Markdown path. Defaults to artifacts/demo-evidence.md.",
    )
    parser.add_argument(
        "--coverage-path",
        default=str(DEFAULT_COVERAGE_PATH),
        help="Coverage Markdown path. Defaults to artifacts/coverage-report.md.",
    )
    parser.add_argument("--minio-user", default=DEFAULT_MINIO_USER, help="MinIO console username.")
    parser.add_argument("--minio-password", default=DEFAULT_MINIO_PASSWORD, help="MinIO console password.")
    parser.add_argument(
        "--output-dir",
        default=str(DEFAULT_OUTPUT_DIR),
        help="Directory for PNG screenshots. Defaults to artifacts/submission-screenshots.",
    )
    parser.add_argument(
        "--browser-channel",
        default="auto",
        help="Browser channel for Playwright. Defaults to auto, trying msedge, chrome, then Playwright Chromium.",
    )
    args = parser.parse_args()

    output_dir = Path(args.output_dir)
    if not output_dir.is_absolute():
        output_dir = ROOT / output_dir
    evidence_path = Path(args.evidence_path)
    if not evidence_path.is_absolute():
        evidence_path = ROOT / evidence_path
    coverage_path = Path(args.coverage_path)
    if not coverage_path.is_absolute():
        coverage_path = ROOT / coverage_path

    try:
        asyncio.run(
            main_async(
                output_dir=output_dir,
                base_url=args.web_base.rstrip("/"),
                minio_base=args.minio_console_base.rstrip("/"),
                evidence_path=evidence_path,
                coverage_path=coverage_path,
                pages=PAGES,
                channel=args.browser_channel,
                minio_user=args.minio_user,
                minio_password=args.minio_password,
            )
        )
    except RuntimeError as exc:
        print(str(exc))
        return 1
    print(f"Wrote submission screenshots to {output_dir}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
