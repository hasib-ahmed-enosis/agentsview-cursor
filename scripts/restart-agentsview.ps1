#Requires -Version 5.1
<#
.SYNOPSIS
  Stop any running agentsview server/daemon and start a fresh foreground serve.

.DESCRIPTION
  Prefers agentsview.exe in the repo root (after scripts/build-windows.ps1).
  Falls back to agentsview on PATH (e.g. ~/.agentsview/bin).
#>
param(
    [string]$Binary = "",
    [string[]]$ServeArgs = @("serve", "--no-browser")
)

$ErrorActionPreference = "Stop"

$RepoRoot = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
if ($Binary) {
    $exe = $Binary
} elseif (Test-Path (Join-Path $RepoRoot "agentsview.exe")) {
    $exe = Join-Path $RepoRoot "agentsview.exe"
} else {
    $onPath = Get-Command agentsview -ErrorAction SilentlyContinue
    if (-not $onPath) {
        throw "No agentsview.exe found. Build with scripts/build-windows.ps1 or install agentsview."
    }
    $exe = $onPath.Source
}

function Stop-Agentsview {
    & $exe daemon stop 2>$null | Out-Null
    & $exe serve stop 2>$null | Out-Null
    Get-Process -Name "agentsview" -ErrorAction SilentlyContinue | Stop-Process -Force -ErrorAction SilentlyContinue
    Start-Sleep -Seconds 1
}

Stop-Agentsview

Write-Host "Starting $exe $($ServeArgs -join ' ')"
Start-Process -FilePath $exe -ArgumentList $ServeArgs -WorkingDirectory $RepoRoot
Start-Sleep -Seconds 2
& $exe serve status 2>&1
