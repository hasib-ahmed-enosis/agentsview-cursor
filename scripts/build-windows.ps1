#Requires -Version 5.1
<#
.SYNOPSIS
  Build agentsview on Windows (frontend embed + Go binary).

.PARAMETER BackendOnly
  Skip the frontend; only compile agentsview.exe (placeholder UI stays embedded).

.PARAMETER FreshNpm
  Run npm ci before npm run build (use after pulling package-lock changes).

.PARAMETER Restart
  Restart agentsview after a successful build (scripts/restart-agentsview.ps1).
#>
param(
    [switch]$BackendOnly,
    [switch]$FreshNpm,
    [switch]$Restart
)

$ErrorActionPreference = "Stop"

function Initialize-UserPath {
    $env:Path = [Environment]::GetEnvironmentVariable("Path", "Machine") + ";" +
        [Environment]::GetEnvironmentVariable("Path", "User")
}

function Build-And-Embed-Frontend {
    param([string]$RepoRoot)

    Initialize-UserPath
    $npm = Get-Command npm -ErrorAction Stop
    $frontend = Join-Path $RepoRoot "frontend"
    $embed = Join-Path $RepoRoot "internal\web\dist"
    $built = Join-Path $frontend "dist\index.html"

    Push-Location $frontend
    try {
        $nodeModules = Join-Path $frontend "node_modules"
        if ($FreshNpm -or -not (Test-Path $nodeModules)) {
            Write-Host "Running npm ci in frontend..."
            & $npm.Source ci
        }
        Write-Host "Running npm run build in frontend..."
        & $npm.Source run build
    } finally {
        Pop-Location
    }

    if (-not (Test-Path $built)) {
        throw "Frontend build did not produce $built"
    }

    if (-not (Test-Path $embed)) {
        New-Item -ItemType Directory -Path $embed | Out-Null
    }
    Get-ChildItem -LiteralPath $embed |
        Where-Object { $_.Name -ne ".keep" } |
        Remove-Item -Recurse -Force
    Copy-Item -Recurse -Force (Join-Path $frontend "dist\*") $embed
    Set-Content -Path (Join-Path $embed ".keep") -Value "keep embed dir for generated frontend assets" -NoNewline
    Write-Host "Embedded frontend assets under internal/web/dist/"
}

$RepoRoot = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
$MsysShell = "C:\msys64\msys2_shell.cmd"
if (-not (Test-Path $MsysShell)) {
    throw "MSYS2 not found at C:\msys64"
}

if (-not $BackendOnly) {
    Build-And-Embed-Frontend -RepoRoot $RepoRoot
}

$Gcc = Join-Path $RepoRoot ".toolchain\ucrt64-gcc14\ucrt64\bin\gcc.exe"
if (-not (Test-Path $Gcc)) {
    Write-Host "Running toolchain setup (one-time)..."
    & (Join-Path $PSScriptRoot "setup-windows-toolchain.ps1")
}

$buildSh = Join-Path $PSScriptRoot "build-windows-msys.sh"
& $MsysShell -ucrt64 -defterm -no-start -here -c "bash '$($buildSh -replace '\\', '/')'"
Write-Host "Built $RepoRoot\agentsview.exe"

if ($Restart) {
    & (Join-Path $PSScriptRoot "restart-agentsview.ps1")
}
