# Dev server for PowerShell
$ErrorActionPreference = "Stop"
Set-Location $PSScriptRoot

function Get-DevPort {
    $defaultPort = 61333
    $configPath = Join-Path $PSScriptRoot "server.yaml"
    if (-not (Test-Path $configPath)) { return $defaultPort }
    $yaml = Get-Content $configPath -Raw
    if ($yaml -match 'addr:\s*":(\d+)"') { return [int]$Matches[1] }
    return $defaultPort
}

function Test-IsKanbanProcess {
    param([int]$ProcessId, [string]$ProjectRoot)
    $proc = Get-Process -Id $ProcessId -ErrorAction SilentlyContinue
    if (-not $proc) { return $false }

    $name = $proc.ProcessName.ToLowerInvariant()
    if ($name -in @('kanban', 'kanban233')) { return $true }

    try {
        if ($proc.Path -and $proc.Path.StartsWith($ProjectRoot, [StringComparison]::OrdinalIgnoreCase)) {
            return $true
        }
    } catch { }

    try {
        $wmi = Get-CimInstance Win32_Process -Filter "ProcessId = $ProcessId" -ErrorAction Stop
        $cmd = $wmi.CommandLine
        if ([string]::IsNullOrWhiteSpace($cmd)) { return $false }
        if ($cmd -match 'kanban233') { return $true }
        if ($cmd -match 'cmd[\\/]kanban') { return $true }
        if ($name -eq 'go' -and $cmd -match 'kanban') { return $true }
    } catch { }

    return $false
}

function Stop-KanbanListenerOnPort {
    param([int]$Port, [string]$ProjectRoot)
    $listeners = @(Get-NetTCPConnection -LocalPort $Port -State Listen -ErrorAction SilentlyContinue)
    if ($listeners.Count -eq 0) { return }

    foreach ($conn in $listeners) {
        $ownerPid = $conn.OwningProcess
        if (Test-IsKanbanProcess -ProcessId $ownerPid -ProjectRoot $ProjectRoot) {
            Write-Host "[dev] port $Port in use by kanban (PID $ownerPid), stopping..."
            Stop-Process -Id $ownerPid -Force -ErrorAction Stop
            Start-Sleep -Milliseconds 400
            continue
        }
        $other = Get-Process -Id $ownerPid -ErrorAction SilentlyContinue
        $label = if ($other) { $other.ProcessName } else { "unknown" }
        throw "port $Port already in use by $label (PID $ownerPid), not kanban233 — stop it manually"
    }
}

if (-not (Test-Path "data")) {
    New-Item -ItemType Directory -Path "data" | Out-Null
}

$port = Get-DevPort
$env:KANBAN_CONFIG = Join-Path $PSScriptRoot "server.yaml"
$env:KANBAN_DEV = "1"

Stop-KanbanListenerOnPort -Port $port -ProjectRoot $PSScriptRoot

Write-Host "[dev] starting kanban233 on http://localhost:$port (web hot reload on)"
go run .\cmd\kanban -config $env:KANBAN_CONFIG
