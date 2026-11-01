#Requires -Version 5.1
$ErrorActionPreference = "Stop"
Set-Location $PSScriptRoot

$env:Path = "C:\Program Files\Go\bin;" + $env:USERPROFILE + "\.cargo\bin;" + $env:Path

if (-not (Get-Command go -ErrorAction SilentlyContinue)) {
    throw "Go não encontrado. Instale: winget install GoLang.Go"
}
if (-not (Get-Command cargo -ErrorAction SilentlyContinue)) {
    throw "Rust não encontrado. Instale: winget install Rustlang.Rustup"
}
# MSVC exige Visual Studio Build Tools; GNU usa o MinGW do WinLibs.
if (-not (rustup show active-toolchain 2>$null | Select-String "gnu")) {
    Write-Host ">> Configurando Rust GNU (MinGW)..." -ForegroundColor DarkGray
    rustup toolchain install stable-x86_64-pc-windows-gnu 2>$null | Out-Null
    rustup default stable-x86_64-pc-windows-gnu | Out-Null
}
if (-not (Get-Command gcc -ErrorAction SilentlyContinue)) {
    throw "GCC (MinGW) não encontrado. Instale: winget install BrechtSanders.WinLibs.POSIX.UCRT"
}

$env:CGO_ENABLED = "1"
$env:CC = "gcc"

$env:CARGO_TARGET_DIR = Join-Path $PSScriptRoot "target"

Write-Host ">> Compilando redirector (Rust)..." -ForegroundColor Cyan
cargo build --release
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

Write-Host ">> Compilando DiscordTunnel (Go + Fyne)..." -ForegroundColor Cyan
Remove-Item -ErrorAction SilentlyContinue cmd\app\rsrc_windows_amd64.syso
go mod tidy
Remove-Item -ErrorAction SilentlyContinue cmd\app\*.syso*
go build -ldflags="-H windowsgui" -o dist\DiscordTunnel.exe .\cmd\app
if ($LASTEXITCODE -ne 0) { throw "go build falhou (exit $LASTEXITCODE)" }

$dist = Join-Path $PSScriptRoot "dist"
New-Item -ItemType Directory -Force -Path $dist | Out-Null
Copy-Item "target\release\redirector.exe" $dist -Force
Copy-Item "target\release\WinDivert.dll" $dist -ErrorAction SilentlyContinue
Copy-Item "target\release\WinDivert64.sys" $dist -ErrorAction SilentlyContinue
if (-not (Test-Path (Join-Path $dist "WinDivert.dll"))) {
    Copy-Item "third_party\openvpn-split\vendor\windivert\WinDivert.dll" $dist -Force
    Copy-Item "third_party\openvpn-split\vendor\windivert\WinDivert64.sys" $dist -Force
}

Write-Host ""
Write-Host "OK  dist\DiscordTunnel.exe + redirector.exe" -ForegroundColor Green
Write-Host "    Para instalador público: .\installer\build-installer.ps1" -ForegroundColor Yellow
