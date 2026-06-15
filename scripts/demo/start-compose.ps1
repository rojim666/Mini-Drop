param(
    [int]$ApiPort = $(if ($env:MINIDROP_API_PORT) { [int]$env:MINIDROP_API_PORT } else { 8080 }),
    [int]$WebPort = $(if ($env:MINIDROP_WEB_PORT) { [int]$env:MINIDROP_WEB_PORT } else { 4173 }),
    [int]$MinioPort = $(if ($env:MINIDROP_MINIO_PORT) { [int]$env:MINIDROP_MINIO_PORT } else { 9000 }),
    [int]$MinioConsolePort = $(if ($env:MINIDROP_MINIO_CONSOLE_PORT) { [int]$env:MINIDROP_MINIO_CONSOLE_PORT } else { 9001 }),
    [int]$SmokeTaskCount = $(if ($env:MINIDROP_COMPOSE_SMOKE_TASKS) { [int]$env:MINIDROP_COMPOSE_SMOKE_TASKS } else { 2 }),
    [switch]$SkipSmoke
)

$ErrorActionPreference = "Stop"

$Root = (Resolve-Path (Join-Path $PSScriptRoot "..\..")).Path
$ComposeFile = Join-Path $Root "deploy\docker-compose.yml"

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

Require-Command -Name "docker" -Hint "Install Docker Desktop and make sure docker.exe is on PATH."
Require-Command -Name "python" -Hint "Install Python 3 and make sure python.exe is on PATH."

docker compose version | Out-Null

$environment = @{
    "MINIDROP_API_PORT" = $ApiPort
    "MINIDROP_WEB_PORT" = $WebPort
    "MINIDROP_MINIO_PORT" = $MinioPort
    "MINIDROP_MINIO_CONSOLE_PORT" = $MinioConsolePort
    "MINIDROP_API_BASE_URL" = "http://127.0.0.1:$ApiPort"
    "MINIDROP_EXPECT_MINIO_URL" = "1"
}

$previous = Set-ScopedEnvironment -Environment $environment
try {
    Invoke-CheckedNative -FilePath "docker" -Arguments @("compose", "-f", $ComposeFile, "up", "--build", "--pull", "never", "-d")

    if (-not $SkipSmoke) {
        if ($SmokeTaskCount -lt 1) {
            throw "SmokeTaskCount must be at least 1 when smoke testing is enabled."
        }

        for ($index = 1; $index -le $SmokeTaskCount; $index++) {
            Write-Host "Running compose smoke task $index/$SmokeTaskCount..."
            Invoke-CheckedNative -FilePath "python" -Arguments @(
                (Join-Path $Root "scripts\demo\smoke_compose.py"),
                "--pid",
                "1",
                "--agent-id",
                "agt_compose",
                "--expect-minio-url"
            )
        }
    }

    Write-Host ""
    Write-Host "Mini-Drop compose demo is ready."
    Write-Host "Web UI:        http://localhost:$WebPort"
    Write-Host "API health:    http://localhost:$ApiPort/healthz"
    Write-Host "MinIO console: http://localhost:$MinioConsolePort"
    Write-Host "MinIO login:   minidrop / minidrop123"
    Write-Host ""
    Write-Host "Use PID 1 in the Web task form for the bundled compose target."
    Write-Host "Snapshot:      .\scripts\demo\acceptance-snapshot.ps1"
    Write-Host "Stop command:  .\scripts\demo\stop-compose.ps1"
} finally {
    Restore-ScopedEnvironment -Previous $previous
}
