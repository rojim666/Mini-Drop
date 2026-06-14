param(
    [string]$ApiAddr = "127.0.0.1:8080",
    [string]$WebAddr = "127.0.0.1",
    [int]$WebPort = 5173
)

$ErrorActionPreference = "Stop"

$Root = (Resolve-Path (Join-Path $PSScriptRoot "..\..")).Path
$LogDir = Join-Path $Root "tmp\local-demo"
New-Item -ItemType Directory -Force -Path $LogDir | Out-Null

function Require-Command {
    param(
        [string]$Name,
        [string]$Hint
    )

    if ($null -eq (Get-Command $Name -ErrorAction SilentlyContinue)) {
        throw "Missing required command '$Name'. $Hint"
    }
}

function Test-TcpPortOpen {
    param(
        [string]$HostName,
        [int]$Port
    )

    $client = [System.Net.Sockets.TcpClient]::new()
    try {
        $async = $client.BeginConnect($HostName, $Port, $null, $null)
        if (-not $async.AsyncWaitHandle.WaitOne(300)) {
            return $false
        }
        $client.EndConnect($async)
        return $true
    } catch {
        return $false
    } finally {
        $client.Close()
    }
}

function Require-PortFree {
    param(
        [string]$Name,
        [string]$HostName,
        [int]$Port
    )

    if (Test-TcpPortOpen -HostName $HostName -Port $Port) {
        throw "$Name port is already in use at ${HostName}:${Port}. Run .\scripts\demo\stop-local.ps1 first, or choose another port."
    }
}

function Split-HostPort {
    param([string]$Address)

    $lastColon = $Address.LastIndexOf(":")
    if ($lastColon -lt 0) {
        throw "Address must be host:port, got '$Address'"
    }

    return @{
        Host = $Address.Substring(0, $lastColon)
        Port = [int]$Address.Substring($lastColon + 1)
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

function Start-DemoProcess {
    param(
        [string]$Name,
        [string]$FilePath,
        [string[]]$Arguments = @(),
        [string]$WorkingDirectory,
        [hashtable]$Environment = @{}
    )

    $stdout = Join-Path $LogDir "$Name.log"
    $stderr = Join-Path $LogDir "$Name.err.log"
    Remove-Item $stdout, $stderr -Force -ErrorAction SilentlyContinue

    $previous = Set-ScopedEnvironment -Environment $Environment
    try {
        $startParams = @{
            FilePath               = $FilePath
            WorkingDirectory       = $WorkingDirectory
            RedirectStandardOutput = $stdout
            RedirectStandardError  = $stderr
            WindowStyle            = "Hidden"
            PassThru               = $true
        }
        if ($Arguments.Count -gt 0) {
            $startParams.ArgumentList = $Arguments
        }

        $process = Start-Process @startParams
        $process.Id | Set-Content -Encoding ASCII (Join-Path $LogDir "$Name.pid")
        Write-Host "$Name started, pid=$($process.Id), log=$stdout"
        return $process
    } finally {
        Restore-ScopedEnvironment -Previous $previous
    }
}

function Wait-TargetPid {
    param([System.Diagnostics.Process]$TargetProcess)

    $pidFile = Join-Path $LogDir "target.pid"
    $logFile = Join-Path $LogDir "target.log"
    $deadline = (Get-Date).AddSeconds(20)

    while ((Get-Date) -lt $deadline) {
        if (Test-Path $logFile) {
            $match = Select-String -Path $logFile -Pattern "target_pid=(\d+)" | Select-Object -Last 1
            if ($null -ne $match) {
                $match.Matches[0].Groups[1].Value | Set-Content -Encoding ASCII $pidFile
                return
            }
        }
        if ($TargetProcess.HasExited) {
            throw "Target process exited early; check $logFile and $(Join-Path $LogDir 'target.err.log')."
        }
        Start-Sleep -Milliseconds 300
    }

    $TargetProcess.Id | Set-Content -Encoding ASCII $pidFile
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

    throw "API server did not become ready at $BaseUrl/healthz; check $(Join-Path $LogDir 'api.log')."
}

function Wait-AgentReady {
    param(
        [string]$BaseUrl,
        [string]$AgentId
    )

    $deadline = (Get-Date).AddSeconds(30)
    while ((Get-Date) -lt $deadline) {
        try {
            $response = Invoke-RestMethod -Uri "$BaseUrl/api/v1/agents" -TimeoutSec 2
            foreach ($agent in $response.agents) {
                if ($agent.id -eq $AgentId -and $agent.status -eq "ONLINE") {
                    return
                }
            }
        } catch {
            Start-Sleep -Seconds 1
        }
        Start-Sleep -Seconds 1
    }

    throw "Agent did not become ONLINE within 30s; check $(Join-Path $LogDir 'agent.log')."
}

Require-Command -Name "python" -Hint "Install Python 3 and make sure python.exe is on PATH."
Require-Command -Name "go" -Hint "Install Go and make sure go.exe is on PATH."
Require-Command -Name "npm.cmd" -Hint "Install Node.js/npm and make sure npm.cmd is on PATH."

$apiParts = Split-HostPort -Address $ApiAddr
Require-PortFree -Name "API" -HostName $apiParts.Host -Port $apiParts.Port
Require-PortFree -Name "Web" -HostName $WebAddr -Port $WebPort

$target = Start-DemoProcess -Name "target" -FilePath "python" -Arguments @("scripts/demo/mock_target.py") -WorkingDirectory $Root
Wait-TargetPid -TargetProcess $target

Start-DemoProcess -Name "api" -FilePath "go" -Arguments @("run", "./apps/api-server") -WorkingDirectory $Root -Environment @{
    "MINIDROP_API_ADDR" = $ApiAddr
    "MINIDROP_ALLOWED_ORIGIN" = "http://$WebAddr`:$WebPort,http://localhost:$WebPort,http://127.0.0.1:$WebPort"
} | Out-Null

Wait-ApiReady -BaseUrl "http://$ApiAddr"

$agentId = if ($env:MINIDROP_AGENT_ID) { $env:MINIDROP_AGENT_ID } else { "agt_local" }
Start-DemoProcess -Name "agent" -FilePath "go" -Arguments @("run", "./apps/agent") -WorkingDirectory $Root -Environment @{
    "MINIDROP_API_BASE_URL" = "http://$ApiAddr"
    "MINIDROP_PYTHON_BIN" = "python"
    "MINIDROP_ANALYZER_SCRIPT" = (Join-Path $Root "apps\analyzer\main.py")
    "MINIDROP_ARTIFACT_DIR" = (Join-Path $Root "artifacts")
    "MINIDROP_AGENT_ID" = $agentId
    "MINIDROP_AGENT_HOSTNAME" = "windows-agent"
    "MINIDROP_AGENT_IP" = "127.0.0.1"
    "MINIDROP_AGENT_VERSION" = "0.1.0"
} | Out-Null

Wait-AgentReady -BaseUrl "http://$ApiAddr" -AgentId $agentId

Start-DemoProcess -Name "web" -FilePath "npm.cmd" -Arguments @("run", "dev", "--", "--host", $WebAddr, "--port", [string]$WebPort) -WorkingDirectory (Join-Path $Root "apps\web") -Environment @{
    "VITE_API_BASE_URL" = "http://$ApiAddr"
} | Out-Null

$targetPid = Get-Content (Join-Path $LogDir "target.pid")
Write-Host ""
Write-Host "Mini-Drop local demo is starting."
Write-Host "Web UI: http://localhost:$WebPort"
Write-Host "API:    http://$ApiAddr/healthz"
Write-Host "Logs:   $LogDir"
Write-Host ""
Write-Host "Use this target PID in the Web form:"
Write-Host $targetPid
Write-Host ""
Write-Host "Smoke helper:"
Write-Host "python scripts\demo\smoke_e2e.py $targetPid $agentId mock-perf"
