param(
    [int]$WebPort = $(if ($env:MINIDROP_WEB_PORT) { [int]$env:MINIDROP_WEB_PORT } else { 80 }),
    [int]$MinioConsolePort = $(if ($env:MINIDROP_MINIO_CONSOLE_PORT) { [int]$env:MINIDROP_MINIO_CONSOLE_PORT } else { 9001 }),
    [string]$OutputDir = $(if ($env:MINIDROP_SUBMISSION_SCREENSHOT_DIR) { $env:MINIDROP_SUBMISSION_SCREENSHOT_DIR } else { "artifacts\submission-screenshots" }),
    [string]$EvidencePath = $(if ($env:MINIDROP_DEMO_EVIDENCE_OUTPUT) { $env:MINIDROP_DEMO_EVIDENCE_OUTPUT } else { "artifacts\demo-evidence.md" }),
    [string]$CoveragePath = $(if ($env:MINIDROP_COVERAGE_OUTPUT) { $env:MINIDROP_COVERAGE_OUTPUT } else { "artifacts\coverage-report.md" }),
    [string]$BrowserChannel = $(if ($env:MINIDROP_SCREENSHOT_BROWSER_CHANNEL) { $env:MINIDROP_SCREENSHOT_BROWSER_CHANNEL } else { "auto" })
)

$ErrorActionPreference = "Stop"

$Root = (Resolve-Path (Join-Path $PSScriptRoot "..\..")).Path

if ($null -eq (Get-Command "python" -ErrorAction SilentlyContinue)) {
    throw "Missing required command 'python'. Install Python 3 and make sure python.exe is on PATH."
}

& python `
    (Join-Path $Root "scripts\demo\capture_submission_artifacts.py") `
    "--web-base" ("http://localhost:{0}" -f $WebPort) `
    "--minio-console-base" ("http://localhost:{0}" -f $MinioConsolePort) `
    "--output-dir" $OutputDir `
    "--evidence-path" $EvidencePath `
    "--coverage-path" $CoveragePath `
    "--browser-channel" $BrowserChannel

if ($LASTEXITCODE -ne 0) {
    throw "capture_submission_artifacts.py exited with code $LASTEXITCODE"
}
