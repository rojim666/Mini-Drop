$ErrorActionPreference = "Continue"

$Root = Resolve-Path (Join-Path $PSScriptRoot "..\..")
$LogDir = Join-Path $Root "tmp\local-demo"
$Names = @("web", "agent", "api", "target")

function Stop-ProcessTree {
    param([int]$ProcessId)

    $children = Get-CimInstance Win32_Process -Filter "ParentProcessId = $ProcessId" -ErrorAction SilentlyContinue
    foreach ($child in $children) {
        Stop-ProcessTree -ProcessId ([int]$child.ProcessId)
    }

    Stop-Process -Id $ProcessId -Force -ErrorAction SilentlyContinue
}

foreach ($name in $Names) {
    $pidFile = Join-Path $LogDir "$name.pid"
    if (Test-Path $pidFile) {
        $procId = [int](Get-Content $pidFile)
        Stop-ProcessTree -ProcessId $procId
        Remove-Item $pidFile -Force -ErrorAction SilentlyContinue
        Write-Host "$name stopped"
    }
}
