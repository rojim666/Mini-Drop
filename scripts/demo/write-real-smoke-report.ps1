param(
    [string[]]$Collectors = $(if ($env:MINIDROP_REAL_COLLECTORS) { $env:MINIDROP_REAL_COLLECTORS } elseif ($env:COLLECTOR_TYPE) { $env:COLLECTOR_TYPE } else { "perf,ebpf-syscall,py-spy" }),
    [string]$Output = $(if ($env:MINIDROP_REAL_SMOKE_OUTPUT) { $env:MINIDROP_REAL_SMOKE_OUTPUT } else { "artifacts/real-smoke-report.md" }),
    [string]$ApiBase = $(if ($env:MINIDROP_API_BASE_URL) { $env:MINIDROP_API_BASE_URL } else { "http://127.0.0.1:8080" }),
    [string]$AgentId = $(if ($env:MINIDROP_AGENT_ID) { $env:MINIDROP_AGENT_ID } else { "agt_local" }),
    [int]$TargetPid = $(if ($env:MINIDROP_TARGET_PID) { [int]$env:MINIDROP_TARGET_PID } else { 0 }),
    [switch]$SkipSmoke,
    [switch]$NoWsl,
    [switch]$Strict
)

$ErrorActionPreference = "Stop"

$Root = (Resolve-Path (Join-Path $PSScriptRoot "..\..")).Path
$scriptPath = Join-Path $Root "scripts\demo\write_real_smoke_report.py"
$collectorValue = ($Collectors | ForEach-Object { $_.Trim() } | Where-Object { $_ }) -join ","

if ($null -eq (Get-Command "python" -ErrorAction SilentlyContinue)) {
    throw "Missing required command 'python'. Install Python 3 and make sure python.exe is on PATH."
}

$argsList = @(
    "--collectors", $collectorValue,
    "--output", $Output,
    "--api-base", $ApiBase,
    "--agent-id", $AgentId
)
if ($TargetPid -gt 0) {
    $argsList += @("--pid", ([string]$TargetPid))
}
if ($SkipSmoke) {
    $argsList += "--skip-smoke"
}
if (-not $Strict) {
    $argsList += "--allow-blocked"
}

function Convert-ToWorkspaceRelativePath {
    param([string]$PathValue)

    if ([string]::IsNullOrWhiteSpace($PathValue)) {
        return $PathValue
    }

    $normalizedRoot = $Root.TrimEnd("\", "/")
    $normalizedPath = $PathValue.Replace("/", "\")
    if ([System.IO.Path]::IsPathRooted($normalizedPath) -and $normalizedPath.StartsWith($normalizedRoot, [System.StringComparison]::OrdinalIgnoreCase)) {
        return $normalizedPath.Substring($normalizedRoot.Length).TrimStart("\").Replace("\", "/")
    }
    return $PathValue
}

$relativeArgs = @()
for ($index = 0; $index -lt $argsList.Count; $index++) {
    $current = $argsList[$index]
    if ($current -eq "--output" -and ($index + 1) -lt $argsList.Count) {
        $relativeArgs += $current
        $relativeArgs += (Convert-ToWorkspaceRelativePath -PathValue $argsList[$index + 1])
        $index++
    } else {
        $relativeArgs += $current
    }
}

$isWindowsHost = $true
if ($PSVersionTable.PSVersion.Major -ge 6) {
    $isWindowsHost = $IsWindows
}

if (-not $NoWsl -and $isWindowsHost -and $null -ne (Get-Command "wsl" -ErrorAction SilentlyContinue)) {
    $wslRoot = "/mnt/" + $Root.Substring(0, 1).ToLowerInvariant() + $Root.Substring(2).Replace("\", "/")
    $quotedArgs = @($relativeArgs | ForEach-Object { "'" + $_.Replace("'", "'\''") + "'" }) -join " "
    $command = "cd '$wslRoot' && python3 scripts/demo/write_real_smoke_report.py $quotedArgs"
    & wsl -e bash -lc $command
} else {
    & python $scriptPath @relativeArgs
}

$exitCode = $LASTEXITCODE
if ($exitCode -ne 0 -and $Strict) {
    throw "write_real_smoke_report.py exited with code $exitCode"
}

if ($exitCode -ne 0) {
    Write-Warning "Real smoke is blocked or failed; see $Output for the generated report."
}
