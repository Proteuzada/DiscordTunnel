# DiscordTunnel

Programa para **Windows** que usa a rede [RiseupVPN](https://riseup.net/vpn) (cliente aberto `bitmask-vpn`) e envia **somente o tráfego do Discord** pelo túnel. O resto do computador continua na rota normal da operadora.

## Para quem só quer usar (sem programar)

1. Baixe **`DiscordTunnel-Setup-1.0.6.exe`** (instalador) na página de [Releases](../../releases) ou em `installer\Output\` depois de compilar.
2. Execute o instalador **como administrador**.
3. Se o PC não tiver OpenVPN, o instalador instala o **OpenVPN Community** com driver **Wintun** (silencioso).
4. Abra **DiscordTunnel** no menu Iniciar e clique em **Conectar**.
5. Use o Discord normalmente.

Requisitos: Windows 10 ou superior, Discord instalado, conexão com a internet.

A RiseupVPN é um serviço mantido por doação — considere [contribuir](https://riseup.net/vpn/donate) se usar com frequência.

Documento técnico: [ROTEAMENTO-DISCORD.md](./ROTEAMENTO-DISCORD.md).

## Compilar o instalador (desenvolvedores)

Ferramentas (uma vez):

```powershell
winget install GoLang.Go Rustlang.Rustup BrechtSanders.WinLibs.POSIX.UCRT JRSoftware.InnoSetup
rustup default stable
```

Gerar o `.exe` de instalação:

```powershell
cd C:\Users\mello\Tunneling
.\installer\build-installer.ps1
```

Saída: `installer\Output\DiscordTunnel-Setup-1.0.6.exe` (app + redirector + WinDivert; OpenVPN embutido no instalador).

## Estrutura

| Pasta | Função |
|-------|--------|
| `cmd/app` | Interface |
| `internal/*` | VPN Riseup, rotas, sessão |
| `third_party/openvpn-split/redirector` | Redirecionamento por processo |
| `installer/` | Inno Setup + download do MSI OpenVPN |

## Licença

MIT — ver [LICENSE.txt](./LICENSE.txt). Componentes de terceiros (WinDivert, redirector) têm licenças próprias.
