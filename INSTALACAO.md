# Instalação para usuários finais

## Um clique (recomendado)

1. Baixe **`DiscordTunnel-Setup-1.0.5.exe`**:
   - Caminho após compilar: `installer\Output\DiscordTunnel-Setup-1.0.5.exe`
   - Ou a release publicada no GitHub (quando disponível).
2. Clique com o botão direito → **Executar como administrador** (ou aceite o UAC quando o instalador pedir).
3. Siga o assistente (português ou inglês).
4. Se o OpenVPN ainda não estiver no PC, o instalador instala automaticamente (componente Wintun).
5. Abra **DiscordTunnel** no Menu Iniciar → **Conectar** → use o Discord.

Não é necessário instalar Go, Rust, Git nem RiseupVPN completa — só este instalador.

## O que o instalador coloca no PC

| Local | Conteúdo |
|-------|----------|
| `C:\Program Files\DiscordTunnel\` | `DiscordTunnel.exe`, `redirector.exe`, WinDivert |
| `%LOCALAPPDATA%\DiscordTunnel\` | Configuração, logs, certificados temporários |
| OpenVPN (se faltava) | `C:\Program Files\OpenVPN\bin\openvpn.exe` |

## Desinstalar

**Configurações → Aplicativos → DiscordTunnel → Desinstalar**

Ou pelo atalho **Desinstalar DiscordTunnel** no Menu Iniciar.

## Problemas comuns

| Sintoma | O que fazer |
|---------|-------------|
| “openvpn.exe não encontrado” | Reinstale e deixe o instalador concluir o passo do OpenVPN |
| “redirector.exe não encontrado” | Reinstale; os quatro arquivos devem estar na mesma pasta |
| Discord não muda de IP | Feche e abra o Discord depois de **Conectar**; confira se o canal (stable/Canary) está marcado |
| Antivírus bloqueia | WinDivert é usado para rotear só o Discord; adicione exceção para a pasta do DiscordTunnel |
| Fica em “conectando” / Wintun em uso | Desative **Cloudflare WARP** (ou outra VPN) e tente de novo — o app usa TAP para não brigar com o Wintun |

## Gerar o instalador de novo (mantenedores)

```powershell
winget install GoLang.Go Rustlang.Rustup BrechtSanders.WinLibs.POSIX.UCRT JRSoftware.InnoSetup
rustup default stable-x86_64-pc-windows-gnu
cd C:\Users\mello\Tunneling
.\installer\build-installer.ps1
```

Saída: `installer\Output\DiscordTunnel-Setup-1.0.5.exe` (~20 MB, inclui OpenVPN embutido).
