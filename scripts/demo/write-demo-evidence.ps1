param(
    [int]$ApiPort = $(if ($env:MINIDROP_API_PORT) { [int]$env:MINIDROP_API_PORT } else { 8080 }),
    [int]$WebPort = $(if ($env:MINIDROP_WEB_PORT) { [int]$env:MINIDROP_WEB_PORT } else { 80 }),
    [int]$MinioPort = $(if ($env:MINIDROP_MINIO_PORT) { [int]$env:MINIDROP_MINIO_PORT } else { 9000 }),
    [string]$Output = $(if ($env:MINIDROP_DEMO_EVIDENCE_OUTPUT) { $env:MINIDROP_DEMO_EVIDENCE_OUTPUT } else { "artifacts\demo-evidence.md" }),
    [switch]$IncludeRealPreflight,
    [string]$RealCollectors = $(if ($env:MINIDROP_REAL_COLLECTORS) { $env:MINIDROP_REAL_COLLECTORS } else { "perf,ebpf-syscall,py-spy" })
)

$ErrorActionPreference = "Stop"

$Root = (Resolve-Path (Join-Path $PSScriptRoot "..\..")).Path

function Require-Command {
    param(
        [string]$Name,
        [string]$Hint
    )

    if ($null -eq (Get-Command $Name -ErrorAction SilentlyContinue)) {
        throw "Missing required command '$Name'. $Hint"
    }
}

function Set-ScopedEnvironment {
    param([hashtable]$Environment)

    $previous = @{}
    foreach ($key in $Environment.Keys) {
        $path = "Env:$key"
        $previous[$key] = if (Test-Path $path) { (Get-Item $path).Value } else { $null }
        Set-Item -Path $path -Value ([string]$Environment[$key])
    }
    return $previous
}

function Restore-ScopedEnvironment {
    param([hashtable]$Previous)

    foreach ($key in $Previous.Keys) {
        $path = "Env:$key"
        if ($null -eq $Previous[$key]) {
            Remove-Item -Path $path -ErrorAction SilentlyContinue
        } else {
            Set-Item -Path $path -Value $Previous[$key]
        }
    }
}

Require-Command -Name "python" -Hint "Install Python 3 and make sure python.exe is on PATH."

$environment = @{
    "MINIDROP_API_PORT" = $ApiPort
    "MINIDROP_WEB_PORT" = $WebPort
    "MINIDROP_MINIO_PORT" = $MinioPort
    "MINIDROP_API_BASE_URL" = "http://127.0.0.1:$ApiPort"
}

$previous = Set-ScopedEnvironment -Environment $environment
try {
    $arguments = @((Join-Path $Root "scripts\demo\write_demo_evidence.py"), "--output", $Output)
    if ($IncludeRealPreflight) {
        $arguments += @("--include-real-preflight", "--real-collectors", $RealCollectors)
    }

    & python @arguments
    if ($LASTEXITCODE -ne 0) {
        throw "write_demo_evidence.py exited with code $LASTEXITCODE"
    }
} finally {
    Restore-ScopedEnvironment -Previous $previous
}
