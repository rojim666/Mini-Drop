param(
    [int]$ApiPort = $(if ($env:MINIDROP_API_PORT) { [int]$env:MINIDROP_API_PORT } else { 8080 }),
    [int]$WebPort = $(if ($env:MINIDROP_WEB_PORT) { [int]$env:MINIDROP_WEB_PORT } else { 80 }),
    [int]$MinioPort = $(if ($env:MINIDROP_MINIO_PORT) { [int]$env:MINIDROP_MINIO_PORT } else { 9000 }),
    [int]$MinioConsolePort = $(if ($env:MINIDROP_MINIO_CONSOLE_PORT) { [int]$env:MINIDROP_MINIO_CONSOLE_PORT } else { 9001 }),
    [switch]$Volumes
)

$ErrorActionPreference = "Stop"

$Root = (Resolve-Path (Join-Path $PSScriptRoot "..\..")).Path
$ComposeFile = Join-Path $Root "deploy\docker-compose.yml"

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

$previous = Set-ScopedEnvironment -Environment @{
    "MINIDROP_API_PORT" = $ApiPort
    "MINIDROP_WEB_PORT" = $WebPort
    "MINIDROP_MINIO_PORT" = $MinioPort
    "MINIDROP_MINIO_CONSOLE_PORT" = $MinioConsolePort
}

try {
    $args = @("compose", "-f", $ComposeFile, "down", "--remove-orphans")
    if ($Volumes) {
        $args += "--volumes"
    }
    Invoke-CheckedNative -FilePath "docker" -Arguments $args
} finally {
    Restore-ScopedEnvironment -Previous $previous
}
