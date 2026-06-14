param(
    [int]$ApiPort = $(if ($env:MINIDROP_API_PORT) { [int]$env:MINIDROP_API_PORT } else { 8080 }),
    [int]$WebPort = $(if ($env:MINIDROP_WEB_PORT) { [int]$env:MINIDROP_WEB_PORT } else { 4173 }),
    [int]$MinioPort = $(if ($env:MINIDROP_MINIO_PORT) { [int]$env:MINIDROP_MINIO_PORT } else { 9000 }),
    [int]$MinioConsolePort = $(if ($env:MINIDROP_MINIO_CONSOLE_PORT) { [int]$env:MINIDROP_MINIO_CONSOLE_PORT } else { 9001 })
)

$ErrorActionPreference = "Stop"

$Root = (Resolve-Path (Join-Path $PSScriptRoot "..\..")).Path

if ($null -eq (Get-Command "python" -ErrorAction SilentlyContinue)) {
    throw "Missing required command 'python'. Install Python 3 and make sure python.exe is on PATH."
}

& python `
    (Join-Path $Root "scripts\demo\check_compose_stack.py") `
    "--api-port" ([string]$ApiPort) `
    "--web-port" ([string]$WebPort) `
    "--minio-port" ([string]$MinioPort) `
    "--minio-console-port" ([string]$MinioConsolePort)

if ($LASTEXITCODE -ne 0) {
    throw "check_compose_stack.py exited with code $LASTEXITCODE"
}
