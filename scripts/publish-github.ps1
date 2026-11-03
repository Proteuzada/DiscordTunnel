#Requires -Version 5.1
# Publica no GitHub (código + release com instalador). Requer: gh auth login
param(
    [string]$RepoName = "DiscordTunnel",
    [switch]$Private
)

$ErrorActionPreference = "Stop"
$root = Split-Path $PSScriptRoot -Parent
Set-Location $root

$env:Path = [System.Environment]::GetEnvironmentVariable("Path", "Machine") + ";" +
            [System.Environment]::GetEnvironmentVariable("Path", "User")

gh auth status | Out-Null
if ($LASTEXITCODE -ne 0) {
    Write-Host "Faça login no GitHub CLI:" -ForegroundColor Yellow
    gh auth login --hostname github.com --git-protocol https --web
}

$version = "1.0.5"
$setup = "installer\Output\DiscordTunnel-Setup-$version.exe"
if (-not (Test-Path $setup)) {
    Write-Host "Gerando instalador..." -ForegroundColor Cyan
    & "$root\installer\build-installer.ps1"
}

$vis = if ($Private) { "--private" } else { "--public" }
$owner = (gh api user -q .login)
Write-Host "Criando repositório $owner/$RepoName..." -ForegroundColor Cyan
gh repo create $RepoName $vis --source=. --remote=origin --push --description "Discord na RiseupVPN sem túnel no sistema inteiro"

gh release create "v$version" `
    "$setup#DiscordTunnel-Setup-$version.exe" `
    --title "DiscordTunnel $version" `
    --notes "Instalador Windows (~20 MB). Inclui OpenVPN se necessário. Ver INSTALACAO.md."

Write-Host ""
Write-Host "Publicado: https://github.com/$owner/$RepoName" -ForegroundColor Green
Write-Host "Release:   https://github.com/$owner/$RepoName/releases/tag/v$version" -ForegroundColor Green
