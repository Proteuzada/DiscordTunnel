# Roteamento só do Discord pela RiseupVPN

Programa para Windows que usa o cliente aberto da RiseupVPN (`bitmask-vpn`) como túnel, e envia por esse túnel apenas o tráfego do Discord. O restante do computador continua na rota normal da operadora, no mesmo modelo seletivo do ExitLag: o aplicativo escolhido muda de caminho; a rota padrão da máquina não muda.

## Objetivo

Quando o programa está ligado:

- `Discord.exe` e os processos filhos da instalação saem pelo IP da Riseup.
- Navegador, jogos, Windows Update e o resto do sistema saem pelo IP da operadora.
- A rota padrão (`0.0.0.0/0`) continua apontando para o gateway físico.
- Desligar o programa devolve o Discord à rota normal, sem deixar rota, adaptador ou filtro órfão.

O que este programa não é: um otimizador de multipath. O ExitLag escolhe vários caminhos e não troca o IP. Aqui o caminho do Discord é o túnel OpenVPN da Riseup, com a criptografia e o IP de saída desse serviço. O parecido com o ExitLag é o recorte por aplicativo, não a rede interna deles.

## Por que a tabela de rotas não basta

O Windows escolhe a rota pelo destino, não pelo processo. Duas consequências:

1. O cliente oficial da Riseup aceita o `redirect-gateway` empurrado pelo servidor. Isso instala `0.0.0.0/1` e `128.0.0.0/1`, que vencem a rota padrão e carregam a máquina inteira para o túnel. Essas rotas não podem ser instaladas.
2. Mesmo uma lista de IPs do Discord na interface da VPN acertaria todo programa que fale com esses IPs (o navegador em `discord.com`, por exemplo) e erraria os IPs novos de voz, CDN e Cloudflare. O Discord troca de endereço o tempo todo.

O recorte tem de ser por processo: o pacote entra no túnel porque quem o enviou foi o Discord, e sai com o IP da interface VPN. A resposta volta por essa interface e é entregue de novo ao processo.

## Arquitetura

```
Discord.exe (e filhos)
        |
        | sockets TCP e UDP, IPv4 e IPv6
        v
+-------------------+
| Redirecionador    |  identifica o PID e pega só esse tráfego
+-------------------+
        |
        | novo socket preso à interface da VPN (IP_UNICAST_IF)
        v
+-------------------+
| OpenVPN + Wintun  |  túnel sobe, rota padrão intacta
| (binário do       |
|  bitmask-vpn)     |
+-------------------+
        |
        v
  gateway Riseup
        |
        v
  internet (Discord enxerga o IP da Riseup)

Todo o resto  -->  rota padrão  -->  placa física  -->  operadora
```

Quatro processos, um deles com privilégio de administrador:

| Peça | Função |
|---|---|
| `app` | Janela: conectar, desconectar, status, IP de saída do sistema e IP de saída do Discord |
| `vpn` | Sobe o OpenVPN com o material do `bitmask-vpn`, sem tomar a rota padrão |
| `redirector` | Associa o tráfego do Discord à interface da VPN e devolve as respostas |
| `watch` | Acompanha a árvore de processos do Discord e a rota padrão |

`app` é o dono dos outros. Se a janela fechar ou cair, um Job Object do Windows com `KILL_ON_JOB_CLOSE` encerra OpenVPN e redirecionador juntos.

## O que usar do repositório da RiseupVPN

Repositório: [leap/bitmask-vpn](https://0xacab.org/leap/bitmask-vpn). A RiseupVPN é esse cliente com `PROVIDER=riseup`. Não reimplementar o protocolo de autenticação anônima nem a escolha de gateway.

Reaproveitar:

- `providers/riseup/` — CA e definição do provedor (`riseup-ca.crt`, API).
- Cliente da API (bonafide / menshen em `pkg/vpn`) — lista de gateways OpenVPN, certificado anônimo e chave temporária.
- `openvpn.exe` que o build do bitmask já empacota, com adaptador Wintun.
- Seleção de gateway e transporte UDP já feitos por esse código.

O programa chama essa biblioteca (ou o binário helper) para obter gateway, CA e certificado, e ele mesmo monta a linha de comando do OpenVPN. A interface Qt oficial não entra no produto: ela não tem túnel por aplicativo, e o pedido de split tunnel da Riseup segue em aberto.

## O que o modo Discord desliga

O helper de Windows do bitmask acrescenta `--block-outside-dns`. Essa flag usa o WFP para impedir DNS fora do túnel em toda a máquina. No modo Discord ela fica de fora, senão o resto do sistema perde DNS.

O servidor também empurra rota e DNS globais. O cliente local recusa isso:

```
--route-nopull
--pull-filter ignore redirect-gateway
--pull-filter ignore "dhcp-option DNS"
--pull-filter ignore block-outside-dns
--pull-filter ignore "route-ipv6"
```

Efeito esperado depois de conectar:

- Existe interface Wintun com o endereço que o servidor entregou.
- Existe rota `/32` do IP do gateway Riseup pela placa física. Sem ela o próprio OpenVPN entra em loop. O OpenVPN cria essa rota sozinho ao conectar; o programa só confere.
- Não existem `0.0.0.0/1` nem `128.0.0.0/1`.
- O gateway padrão é o mesmo de antes de conectar.
- Os servidores DNS da placa física continuam os da operadora.

Se qualquer uma dessas condições falhar, o programa derruba o OpenVPN e não liga o redirecionador.

No Linux o bitmask tem firewall que corta o que não vai pela VPN. No Windows atual esse firewall do helper não está implementado. Mesmo assim o modo Discord nunca chama `firewallStart`.

## Redirecionador

A tabela de rotas permanece como está. Quem muda o caminho é o redirecionador, no mesmo espírito de um cliente OpenVPN de split por aplicativo: observa o socket, e reabre a conexão presa na interface VPN.

### Como ele reconhece o Discord

Lista inicial, por caminho do executável, não só pelo nome:

- `%LOCALAPPDATA%\Discord\`
- `%LOCALAPPDATA%\DiscordCanary\`
- `%LOCALAPPDATA%\DiscordPTB\`
- `%LOCALAPPDATA%\DiscordDevelopment\`

Inclui processos filhos. A voz do Discord roda dentro dessa árvore (módulo nativo no processo Electron), então TCP do gateway e UDP de voz caem juntos.

`Update.exe` do Squirrel fica de fora: atualização não precisa do túnel e não deve segurar o desligamento.

A admissão do PID acontece em três momentos, porque o Discord abre socket milissegundos depois de nascer:

1. Camada de socket avisa o `connect` já com o PID.
2. Se o pacote chegar antes, consulta a tabela de conexões e compara o caminho do executável.
3. Se ainda não der, o pacote fica retido por no máximo 1 ms e a consulta repete. Não deu: o pacote segue pela rota normal. Nunca é descartado por falta de identificação.

### Como o pacote atravessa

Não se reescreve a rota do pacote original. O fluxo é:

1. O redirecionador vê a conexão de saída de um PID admitido.
2. Segura esse fluxo.
3. Abre outro socket com `IP_UNICAST_IF` / `IPV6_UNICAST_IF` apontando para o índice da interface Wintun.
4. Faz a ponte entre os dois lados (TCP e UDP).
5. A resposta chega no IP da VPN, volta pela ponte e é entregue ao Discord.

Assim a origem que a Riseup enxerga é o IP do túnel, e a origem que o Discord enxerga continua local. Sem esse passo, o pacote sairia com o IP da rede doméstica, a resposta não voltaria pela VPN e a voz cairia.

UDP entra no mesmo caminho. Sem UDP, texto e imagem podem passar e a chamada de voz continua na operadora.

### IPv6

A Riseup, neste uso, é túnel IPv4. Se o Discord resolver AAAA e falar IPv6 pela placa física, a voz foge do túnel. Enquanto o modo Discord estiver ativo, o redirecionador pega também o IPv6 desses PIDs e não o deixa sair pela placa física: ou a conexão é refeita em IPv4 pelo túnel, ou aquele fluxo IPv6 do Discord é recusado. O IPv6 do resto do sistema não é mexido.

### DNS

As consultas DNS feitas pelos PIDs do Discord passam pela mesma ponte, então o endereço que o Discord usa já nasce do lado do túnel. O DNS da placa física não é alterado e `--block-outside-dns` não é ligado. Resolução do navegador e do sistema permanece na operadora.

Há um vazamento que rota nenhuma corrige: o Discord anuncia o IP local dentro do sinal de voz (candidatos ICE do tipo host). O programa não edita o tráfego do Discord; só muda por onde o pacote sai. Quem precisar esconder o IP local da rede tem de tratar isso à parte, ciente de que mexer no payload quebra chamada.

## Subida e descida

### Conectar

1. Gravar rota padrão, índice da placa física, gateway e servidores DNS.
2. Pedir ao código do `bitmask-vpn` um gateway OpenVPN e o certificado anônimo. O certificado fica em diretório temporário e é apagado ao desconectar, como o cliente oficial já faz.
3. Subir `openvpn.exe` com as flags da seção anterior, UDP, Wintun, e o remoto escolhido.
4. Esperar o estado `CONNECTED` no management interface do OpenVPN.
5. Repetir a checagem de rota. Falhou: matar o OpenVPN e mostrar o erro. Passou: anotar o índice da interface Wintun.
6. Só então armar o redirecionador na lista de caminhos do Discord.
7. Medir dois IPs de saída: um socket comum (tem de continuar o IP da operadora) e um socket preso à Wintun (tem de ser o IP da Riseup). Mostrar os dois na janela.

### Desconectar

1. Parar o redirecionador. Conexões do Discord que estavam na ponte caem; o cliente do Discord reconecta sozinho pela rota normal.
2. Encerrar o OpenVPN e esperar a interface sumir.
3. Conferir se a rota padrão gravada no passo 1 voltou. Se sobrou `0.0.0.0/1`, `128.0.0.0/1` ou rota do gateway, apagar.
4. Apagar o diretório temporário do certificado.

### Queda no meio do caminho

Padrão: falha aberta, igual a continuar jogando quando o otimizador cai. Se o OpenVPN sair, o redirecionador para e o Discord segue pela operadora. A janela mostra "Discord na rota normal". Não há kill switch do sistema.

Opção futura, desligada por padrão: falha fechada só para os PIDs do Discord (o aplicativo perde a rede até o túnel voltar). O resto da máquina continua intocado.

## Janela

Estados visíveis: desconectado, conectando, no ar, erro de rota, túnel caiu.

No ar, a janela mostra:

- gateway Riseup (cidade / hostname que a API devolveu)
- IP de saída do sistema
- IP de saída do Discord
- rota padrão (gateway físico) antes e depois, para deixar óbvio que não mudou
- bytes enviados e recebidos só da ponte do Discord
- processos admitidos naquele momento

Ações: conectar, desconectar, abrir a pasta de log. Sem configuração de rota manual na primeira versão. Canary e PTB entram pela lista de caminhos; a janela só deixa desmarcar cada um.

O programa pede administrador uma vez, porque OpenVPN e o redirecionador precisam disso. Não instala serviço permanente: sobe com a janela e morre com ela.

## Layout do código

```
cmd/app/            janela e ciclo de vida (Job Object)
internal/vpn/       embrulha pkg/vpn do bitmask-vpn e a linha de comando
internal/route/     lê e compara a rota padrão; recusa o túnel se ela mudar
internal/watch/     árvore de processos por caminho
internal/redirect/  ponte TCP/UDP presa à interface Wintun
```

O módulo `internal/vpn` declara dependência do checkout do `bitmask-vpn` (substitução Go `replace` apontando para `third_party/bitmask-vpn`). Atualizar a RiseupVPN é atualizar esse checkout e rebuildar. Não copiar a árvore do LEAP para dentro do código próprio.

## Critérios de pronto

1. Com o túnel no ar, `route print` mostra o mesmo gateway padrão de antes, e não mostra `0.0.0.0/1` nem `128.0.0.0/1`.
2. Um teste de IP no navegador mostra o IP da operadora.
3. Uma chamada de voz no Discord, e o tráfego TCP do cliente, saem pelo IP da Riseup. Os dois são medidos na ponte, não só o site.
4. Fechar a janela no meio da chamada encerra OpenVPN e redirecionador em menos de dois segundos, e a rota padrão permanece a original.
5. Matar o `openvpn.exe` no Gerenciador de Tarefas tira o Discord do túnel e o deixa falar pela operadora.
6. Chrome, jogo e `nslookup` do sistema não passam pela interface Wintun (contador da ponte não sobe quando só eles geram tráfego).
7. DNS da placa física não muda durante a sessão.

## Limites

- Exige Windows 10 ou superior, com o driver Wintun que já vem no build do bitmask. Adaptador TAP antigo não é alvo.
- A primeira versão é IPv4 no túnel. IPv6 do Discord é bloqueado só para aqueles PIDs, como descrito acima.
- Não escolhe o servidor de voz do Discord nem faz multipath. A latência é a do gateway Riseup que a API entregar, mais o custo do OpenVPN. Dá para preferir um país na lista de gateways do bitmask; isso não é o mesmo que o ExitLag medir vários caminhos.
- A RiseupVPN é mantida por doação e pensada como túnel de privacidade. Usar só o Discord gasta menos a rede deles do que o túnel cheio, e o uso continua sujeito aos termos do serviço. Vale contribuir se for uso frequente.
- Não edita arquivo do Discord, não injeta DLL no cliente e não mexe em anti-cheat de jogo. O único processo observado por dentro é a lista de PIDs e o caminho do executável.
- Certificado e chave são os anônimos que o próprio bitmask pede à API, vivem em pasta temporária e somem ao desconectar. Não vão para o repositório.

## Referências

- Cliente: https://0xacab.org/leap/bitmask-vpn
- CA e provedor Riseup, dentro desse repositório: `providers/riseup/`
- Pedido em aberto de split tunnel no cliente oficial: https://github.com/riseupnet/riseup_help/issues/650
- O que o `redirect-gateway` faz com a rota padrão: documentação do OpenVPN, "Routing all client traffic through the VPN"
- `--block-outside-dns`: manual do OpenVPN, opções de Windows. É global; por isso fica desligado aqui.
