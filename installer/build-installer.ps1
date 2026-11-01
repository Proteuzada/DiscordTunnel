#Requires -Version 5.1
# Gera dist\ + installer\Output\DiscordTunnel-Setup-1.0.0.exe
param([switch]$SkipAppBuild)

$ErrorActionPreference = "Stop"
$root = Split-Path $PSScriptRoot -Parent

$openvpnVersion = "2.7.4"
$openvpnMsi = "OpenVPN-$openvpnVersion-I001-amd64.msi"
$openvpnUrl = "https://swupdate.openvpn.org/community/releases/$openvpnMsi"
$openvpnDest = Join-Path $PSScriptRoot "deps\$openvpnMsi"

if (-not (Test-Path $openvpnDest)) {
    Write-Host "[deps] Baixando OpenVPN $openvpnVersion..." -ForegroundColor Cyan
    New-Item -ItemType Directory -Path (Split-Path $openvpnDest) -Force | Out-Null
    Invoke-WebRequest -Uri $openvpnUrl -OutFile $openvpnDest -UseBasicParsing
}

if (-not $SkipAppBuild) {
    & (Join-Path $root "build.ps1")
    if ($LASTEXITCODE -ne 0) { throw "build.ps1 falhou" }
}

$distExe = Join-Path $root "dist\DiscordTunnel.exe"
if (-not (Test-Path $distExe)) {
    throw "dist\DiscordTunnel.exe não encontrado. Rode build.ps1 primeiro."
}

$iscc = @(
    "$env:ProgramFiles\Inno Setup 6\ISCC.exe",
    "${env:ProgramFiles(x86)}\Inno Setup 6\ISCC.exe",
    "$env:LOCALAPPDATA\Programs\Inno Setup 6\ISCC.exe"
) | Where-Object { Test-Path $_ } | Select-Object -First 1

if (-not $iscc) {
    throw "Inno Setup 6 não encontrado. Instale: winget install JRSoftware.InnoSetup"
}

Write-Host "[installer] Compilando com Inno Setup..." -ForegroundColor Cyan
& $iscc (Join-Path $PSScriptRoot "DiscordTunnel.iss")
if ($LASTEXITCODE -ne 0) { throw "ISCC falhou" }

$out = Get-ChildItem (Join-Path $PSScriptRoot "Output\*.exe") |
    Sort-Object LastWriteTime -Descending | Select-Object -First 1
Write-Host ""
Write-Host "Instalador pronto:" -ForegroundColor Green
Write-Host "  $($out.FullName)  ($([math]::Round($out.Length / 1MB, 1)) MB)"
