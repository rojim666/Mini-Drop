param(
    [string]$ApiAddr = "127.0.0.1:8080",
    [string]$WebAddr = "127.0.0.1",
    [int]$WebPort = 5173
)

$ErrorActionPreference = "Stop"

$Root = Resolve-Path (Join-Path $PSScriptRoot "..\..")
$LogDir = Join-Path $Root "tmp\local-demo"
New-Item -ItemType Directory -Force -Path $LogDir | Out-Null

function Start-DemoProcess {
    param(
        [string]$Name,
        [string]$FilePath,
        [string[]]$Arguments,
        [string]$WorkingDirectory,
        [hashtable]$Environment = @{}
    )

    $argumentString = ($Arguments | ForEach-Object {
        if ($_ -match '[\s"]') { '"' + ($_ -replace '"', '\"') + '"' } else { $_ }
    }) -join ' '

    $previous = @{}
    foreach ($key in $Environment.Keys) {
        $path = "Env:$key"
        $previous[$key] = if (Test-Path $path) { (Get-Item $path).Value } else { $null }
        Set-Item -Path $path -Value ([string]$Environment[$key])
    }

    try {
        $process = Start-Process `
            -FilePath $FilePath `
            -ArgumentList $argumentString `
            -WorkingDirectory $WorkingDirectory `
            -PassThru
        $process.Id | Set-Content -Encoding ASCII (Join-Path $LogDir "$Name.pid")
        Write-Host "$Name started, pid=$($process.Id)"
    } finally {
        foreach ($key in $Environment.Keys) {
            $path = "Env:$key"
            if ($null -eq $previous[$key]) {
                Remove-Item -Path $path -ErrorAction SilentlyContinue
            } else {
                Set-Item -Path $path -Value $previous[$key]
            }
        }
    }
}

function Wait-ApiReady {
    param([string]$BaseUrl)

    $deadline = (Get-Date).AddSeconds(30)
    while ((Get-Date) -lt $deadline) {
        try {
            Invoke-WebRequest -Uri "$BaseUrl/healthz" -UseBasicParsing -TimeoutSec 2 | Out-Null
            return
        } catch {
            Start-Sleep -Seconds 1
        }
    }

    throw "API server did not become ready at $BaseUrl/healthz"
}

Start-DemoProcess -Name "target" -FilePath "python" -Arguments @("scripts/demo/mock_target.py") -WorkingDirectory $Root
Start-DemoProcess -Name "api" -FilePath "go" -Arguments @("run", "./apps/api-server") -WorkingDirectory $Root -Environment @{
    "MINIDROP_API_ADDR" = $ApiAddr
    "MINIDROP_ALLOWED_ORIGIN" = "http://$WebAddr`:$WebPort,http://localhost:$WebPort,http://127.0.0.1:$WebPort"
}

Wait-ApiReady -BaseUrl "http://$ApiAddr"

Start-DemoProcess -Name "agent" -FilePath "go" -Arguments @("run", "./apps/agent") -WorkingDirectory $Root -Environment @{
    "MINIDROP_API_BASE_URL" = "http://$ApiAddr"
    "MINIDROP_PYTHON_BIN" = "python"
    "MINIDROP_ANALYZER_SCRIPT" = (Join-Path $Root "apps\analyzer\main.py")
    "MINIDROP_ARTIFACT_DIR" = (Join-Path $Root "artifacts")
}

Start-DemoProcess -Name "web" -FilePath "npm.cmd" -Arguments @("run", "dev", "--", "--host", $WebAddr, "--port", [string]$WebPort) -WorkingDirectory (Join-Path $Root "apps\web") -Environment @{
    "VITE_API_BASE_URL" = "http://$ApiAddr"
}

Write-Host ""
Write-Host "Mini-Drop local demo is starting."
Write-Host "Web UI: http://localhost:$WebPort"
Write-Host "API:    http://$ApiAddr/healthz"
Write-Host "Logs:   $LogDir"
Write-Host ""
Write-Host "Use this target PID in the Web form:"
Get-Content (Join-Path $LogDir "target.pid")
