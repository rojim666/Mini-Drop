param(
    [int]$ApiPort = $(if ($env:MINIDROP_API_PORT) { [int]$env:MINIDROP_API_PORT } else { 8080 }),
    [int]$WebPort = $(if ($env:MINIDROP_WEB_PORT) { [int]$env:MINIDROP_WEB_PORT } else { 80 }),
    [int]$MinioPort = $(if ($env:MINIDROP_MINIO_PORT) { [int]$env:MINIDROP_MINIO_PORT } else { 9000 }),
    [int]$MinioConsolePort = $(if ($env:MINIDROP_MINIO_CONSOLE_PORT) { [int]$env:MINIDROP_MINIO_CONSOLE_PORT } else { 9001 }),
    [string]$EvidencePath = $(if ($env:MINIDROP_DEMO_EVIDENCE_OUTPUT) { $env:MINIDROP_DEMO_EVIDENCE_OUTPUT } else { "artifacts\demo-evidence.md" }),
    [string]$AttributionEvaluationPath = $(if ($env:MINIDROP_ATTRIBUTION_EVALUATION_OUTPUT) { $env:MINIDROP_ATTRIBUTION_EVALUATION_OUTPUT } else { "artifacts\attribution-evaluation-report.md" }),
    [string]$Output = $(if ($env:MINIDROP_RECORDING_CHECKLIST_OUTPUT) { $env:MINIDROP_RECORDING_CHECKLIST_OUTPUT } else { "artifacts\recording-checklist.md" })
)

$ErrorActionPreference = "Stop"

$Root = (Resolve-Path (Join-Path $PSScriptRoot "..\..")).Path

if ($null -eq (Get-Command "python" -ErrorAction SilentlyContinue)) {
    throw "Missing required command 'python'. Install Python 3 and make sure python.exe is on PATH."
}

$arguments = @(
    (Join-Path $Root "scripts\demo\write_recording_checklist.py"),
    "--output",
    $Output,
    "--api-port",
    ([string]$ApiPort),
    "--web-port",
    ([string]$WebPort),
    "--minio-port",
    ([string]$MinioPort),
    "--minio-console-port",
    ([string]$MinioConsolePort),
    "--evidence-path",
    $EvidencePath,
    "--attribution-evaluation-path",
    $AttributionEvaluationPath
)

& python @arguments
if ($LASTEXITCODE -ne 0) {
    throw "write_recording_checklist.py exited with code $LASTEXITCODE"
}
