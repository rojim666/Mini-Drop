param(
    [int]$ApiPort = $(if ($env:MINIDROP_API_PORT) { [int]$env:MINIDROP_API_PORT } else { 8080 }),
    [int]$WebPort = $(if ($env:MINIDROP_WEB_PORT) { [int]$env:MINIDROP_WEB_PORT } else { 80 }),
    [int]$MinioPort = $(if ($env:MINIDROP_MINIO_PORT) { [int]$env:MINIDROP_MINIO_PORT } else { 9000 }),
    [int]$SeedTaskCount = $(if ($env:MINIDROP_ACCEPTANCE_SEED_TASKS) { [int]$env:MINIDROP_ACCEPTANCE_SEED_TASKS } else { 2 }),
    [int]$TargetPid = $(if ($env:MINIDROP_TARGET_PID) { [int]$env:MINIDROP_TARGET_PID } else { 1 }),
    [string]$AgentId = $(if ($env:MINIDROP_TARGET_AGENT_ID) { $env:MINIDROP_TARGET_AGENT_ID } else { "drop_agent" }),
    [switch]$SeedTasks
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

function Invoke-CheckedNative {
    param(
        [string]$FilePath,
        [string[]]$Arguments
    )

    & $FilePath @Arguments
    if ($LASTEXITCODE -ne 0) {
        throw "$FilePath $($Arguments -join ' ') exited with code $LASTEXITCODE"
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
    if ($SeedTasks) {
        if ($SeedTaskCount -lt 1) {
            throw "SeedTaskCount must be at least 1 when -SeedTasks is set."
        }

        for ($index = 1; $index -le $SeedTaskCount; $index++) {
            Write-Host "Seeding acceptance task $index/$SeedTaskCount..."
            Invoke-CheckedNative -FilePath "python" -Arguments @(
                (Join-Path $Root "scripts\demo\smoke_compose.py"),
                "--pid",
                ([string]$TargetPid),
                "--agent-id",
                $AgentId,
                "--expect-minio-url"
            )
        }
    }

    Invoke-CheckedNative -FilePath "python" -Arguments @(
        (Join-Path $Root "scripts\demo\acceptance_snapshot.py")
    )
} finally {
    Restore-ScopedEnvironment -Previous $previous
}
