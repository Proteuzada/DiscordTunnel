## DiscordTunnel 1.0.6

O Discord ficava na tela de início porque o DNS dele, enviado ao roteador de casa (`192.168.1.1`), era jogado para dentro do túnel de Miami e nunca recebia resposta. Rede local, link-local e multicast agora seguem pela placa normal. Só o tráfego público do Discord sai pela Riseup.

Nada é gravado na instalação do Discord: ao desconectar, o aplicativo volta a usar a internet da operadora.

Quem já está na 1.0.4 ou 1.0.5 recebe esta versão ao abrir o aplicativo. Depois de conectar, feche o Discord por completo e abra de novo.
