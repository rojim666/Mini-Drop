param(
    [string]$Collectors = $(if ($env:MINIDROP_REAL_COLLECTORS) { $env:MINIDROP_REAL_COLLECTORS } else { "perf,ebpf-syscall,py-spy" }),
    [int]$TargetPid = $(if ($env:MINIDROP_TARGET_PID) { [int]$env:MINIDROP_TARGET_PID } else { 0 }),
    [string]$Output = $(if ($env:MINIDROP_REAL_COLLECTOR_PREFLIGHT_OUTPUT) { $env:MINIDROP_REAL_COLLECTOR_PREFLIGHT_OUTPUT } else { "artifacts/real-collector-preflight.md" }),
    [switch]$Install,
    [switch]$NoWsl,
    [switch]$Strict
)

$ErrorActionPreference = "Stop"

$Root = (Resolve-Path (Join-Path $PSScriptRoot "..\..")).Path
$scriptPath = Join-Path $Root "scripts\demo\prepare_real_collectors.py"

$arguments = @(
    "--collectors",
    $Collectors,
    "--output",
    $Output
)

if ($TargetPid -gt 0) {
    $arguments += @("--pid", ([string]$TargetPid))
}

if ($Install) {
    $arguments += "--install"
}

$isWindowsHost = $true
if ($PSVersionTable.PSVersion.Major -ge 6) {
    $isWindowsHost = $IsWindows
}

if (-not $NoWsl -and $isWindowsHost -and $null -ne (Get-Command "wsl" -ErrorAction SilentlyContinue)) {
    $wslRoot = "/mnt/" + $Root.Substring(0, 1).ToLowerInvariant() + $Root.Substring(2).Replace("\", "/")
    $quotedArgs = @($arguments | ForEach-Object { "'" + $_.Replace("'", "'\''") + "'" }) -join " "
    $command = "cd '$wslRoot' && python3 scripts/demo/prepare_real_collectors.py $quotedArgs"
    & wsl -e bash -lc $command
} else {
    if ($null -eq (Get-Command "python" -ErrorAction SilentlyContinue)) {
        throw "Missing required command 'python'. Install Python 3 or run this helper inside WSL2."
    }
    & python $scriptPath @arguments
}

if ($LASTEXITCODE -ne 0) {
    if ($Strict) {
        throw "prepare_real_collectors.py exited with code $LASTEXITCODE"
    }
    Write-Warning "Real collector prerequisites are blocked; see $Output for the generated report."
}
