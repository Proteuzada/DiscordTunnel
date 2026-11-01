//go:build windows

package main

import (
	"os"
	"os/exec"

	"fyne.io/fyne/v2"
	fyneapp "fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"github.com/mello/tunneling/internal/app"
	"github.com/mello/tunneling/internal/config"
	"golang.org/x/sys/windows"
)

func main() {
	if !isAdmin() {
		relaunchElevated()
		return
	}

	settings := config.LoadSettings()
	a := fyneapp.NewWithID("com.mello.discord-tunnel")
	a.SetIcon(nil)
	w := a.NewWindow("DiscordTunnel — RiseupVPN")
	w.Resize(fyne.NewSize(520, 420))

	statusLabel := widget.NewLabel("Desconectado")
	gatewayLabel := widget.NewLabel("Gateway: —")
	sysIPLabel := widget.NewLabel("IP do sistema: —")
	discordIPLabel := widget.NewLabel("IP do Discord (ponte): —")
	routeLabel := widget.NewLabel("Rota padrão: —")
	errLabel := widget.NewLabel("")
	errLabel.Wrapping = fyne.TextWrapWord

	var session *app.Session
	updateUI := func(st app.Status) {
		statusLabel.SetText(string(st.State))
		gatewayLabel.SetText("Gateway Riseup: " + st.Gateway)
		sysIPLabel.SetText("IP do sistema: " + st.SystemIP)
		discordIPLabel.SetText("IP do Discord: " + st.DiscordIP)
		if st.GatewayBefore != "" {
			routeLabel.SetText("Rota padrão: " + st.GatewayBefore + " → " + st.GatewayAfter)
		}
		errLabel.SetText(st.Error)
	}

	uiCh := make(chan app.Status, 16)
	go func() {
		for st := range uiCh {
			st := st
			updateUI(st)
		}
	}()
	session = app.NewSession(settings, func(st app.Status) {
		select {
		case uiCh <- st:
		default:
		}
	})

	stableCheck := widget.NewCheck("Discord stable", func(v bool) { settings.Channels.Stable = v })
	stableCheck.SetChecked(settings.Channels.Stable)
	canaryCheck := widget.NewCheck("Canary", func(v bool) { settings.Channels.Canary = v })
	canaryCheck.SetChecked(settings.Channels.Canary)
	ptbCheck := widget.NewCheck("PTB", func(v bool) { settings.Channels.PTB = v })
	ptbCheck.SetChecked(settings.Channels.PTB)

	connectBtn := widget.NewButton("Conectar", func() {
		_ = config.SaveSettings(settings)
		go session.Connect()
	})
	disconnectBtn := widget.NewButton("Desconectar", func() {
		session.Disconnect()
	})
	openLogsBtn := widget.NewButton("Abrir pasta de log", func() {
		dir, err := config.LogDir()
		if err != nil {
			return
		}
		_ = exec.Command("explorer", dir).Start()
	})

	w.SetContent(container.NewVBox(
		widget.NewLabel("Só o Discord desktop sai pela Riseup. Depois de conectar, feche o Discord por completo e abra de novo (F5 reaproveita a conexão antiga)."),
		statusLabel,
		gatewayLabel,
		sysIPLabel,
		discordIPLabel,
		routeLabel,
		errLabel,
		widget.NewSeparator(),
		widget.NewLabel("Canais do Discord"),
		stableCheck, canaryCheck, ptbCheck,
		container.NewGridWithColumns(2, connectBtn, disconnectBtn),
		openLogsBtn,
	))

	w.SetCloseIntercept(func() {
		session.Disconnect()
		w.Close()
	})

	w.Show()
	a.Run()
}

func isAdmin() bool {
	var token windows.Token
	if err := windows.OpenProcessToken(windows.CurrentProcess(), windows.TOKEN_QUERY, &token); err != nil {
		return false
	}
	defer token.Close()
	return token.IsElevated()
}

func relaunchElevated() {
	exe, err := os.Executable()
	if err != nil {
		return
	}
	verb := "runas"
	cwd, _ := os.Getwd()
	_ = exec.Command("powershell", "-Command",
		"Start-Process -FilePath '"+exe+"' -Verb "+verb+" -WorkingDirectory '"+cwd+"'").Run()
}
