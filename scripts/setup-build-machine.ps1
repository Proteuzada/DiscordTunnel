#Requires -RunAsAdministrator
# Prepara o PC para compilar o DiscordTunnel (não é necessário para quem só instala o .exe)
$ErrorActionPreference = "Stop"

$packages = @(
    "GoLang.Go",
    "Rustlang.Rustup",
    "BrechtSanders.WinLibs.POSIX.UCRT",
    "JRSoftware.InnoSetup"
)
foreach ($p in $packages) {
    Write-Host "Instalando $p..." -ForegroundColor Cyan
    winget install $p --accept-package-agreements --accept-source-agreements --disable-interactivity
}

$env:Path = [System.Environment]::GetEnvironmentVariable("Path", "Machine") + ";" +
            [System.Environment]::GetEnvironmentVariable("Path", "User")
rustup toolchain install stable-x86_64-pc-windows-gnu
rustup default stable-x86_64-pc-windows-gnu

Write-Host ""
Write-Host "Pronto. Feche e reabra o terminal, depois:" -ForegroundColor Green
Write-Host "  cd $PSScriptRoot\.."
Write-Host "  .\installer\build-installer.ps1"
