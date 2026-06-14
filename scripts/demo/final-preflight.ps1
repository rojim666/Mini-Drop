param(
    [int]$ApiPort = $(if ($env:MINIDROP_API_PORT) { [int]$env:MINIDROP_API_PORT } else { 8080 }),
    [int]$WebPort = $(if ($env:MINIDROP_WEB_PORT) { [int]$env:MINIDROP_WEB_PORT } else { 4173 }),
    [int]$MinioPort = $(if ($env:MINIDROP_MINIO_PORT) { [int]$env:MINIDROP_MINIO_PORT } else { 9000 }),
    [int]$MinioConsolePort = $(if ($env:MINIDROP_MINIO_CONSOLE_PORT) { [int]$env:MINIDROP_MINIO_CONSOLE_PORT } else { 9001 }),
    [string]$Output = $(if ($env:MINIDROP_FINAL_PREFLIGHT_OUTPUT) { $env:MINIDROP_FINAL_PREFLIGHT_OUTPUT } else { "artifacts\final-preflight.md" }),
    [switch]$SeedTasks,
    [int]$SeedTaskCount = $(if ($env:MINIDROP_ACCEPTANCE_SEED_TASKS) { [int]$env:MINIDROP_ACCEPTANCE_SEED_TASKS } else { 2 }),
    [int]$TargetPid = $(if ($env:MINIDROP_TARGET_PID) { [int]$env:MINIDROP_TARGET_PID } else { 1 }),
    [string]$AgentId = $(if ($env:MINIDROP_TARGET_AGENT_ID) { $env:MINIDROP_TARGET_AGENT_ID } else { "agt_compose" }),
    [switch]$IncludeRealPreflight,
    [string]$RealCollectors = $(if ($env:MINIDROP_REAL_COLLECTORS) { $env:MINIDROP_REAL_COLLECTORS } else { "perf,ebpf-syscall,py-spy" }),
    [switch]$SkipLive,
    [switch]$SkipTests
)

$ErrorActionPreference = "Stop"

$Root = (Resolve-Path (Join-Path $PSScriptRoot "..\..")).Path

if ($null -eq (Get-Command "python" -ErrorAction SilentlyContinue)) {
    throw "Missing required command 'python'. Install Python 3 and make sure python.exe is on PATH."
}

$arguments = @(
    (Join-Path $Root "scripts\demo\run_final_preflight.py"),
    "--api-port",
    ([string]$ApiPort),
    "--web-port",
    ([string]$WebPort),
    "--minio-port",
    ([string]$MinioPort),
    "--minio-console-port",
    ([string]$MinioConsolePort),
    "--output",
    $Output,
    "--seed-task-count",
    ([string]$SeedTaskCount),
    "--target-pid",
    ([string]$TargetPid),
    "--agent-id",
    $AgentId,
    "--real-collectors",
    $RealCollectors
)

if ($SeedTasks) {
    $arguments += "--seed-tasks"
}

if ($IncludeRealPreflight) {
    $arguments += "--include-real-preflight"
}

if ($SkipLive) {
    $arguments += "--skip-live"
}

if ($SkipTests) {
    $arguments += "--skip-tests"
}

& python @arguments
if ($LASTEXITCODE -ne 0) {
    throw "run_final_preflight.py exited with code $LASTEXITCODE"
}
