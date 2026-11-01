//go:build windows

package app

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/mello/tunneling/internal/config"
	"github.com/mello/tunneling/internal/egress"
	"github.com/mello/tunneling/internal/job"
	"github.com/mello/tunneling/internal/redirect"
	"github.com/mello/tunneling/internal/riseup"
	"github.com/mello/tunneling/internal/route"
	"github.com/mello/tunneling/internal/vpn"
)

type State string

const (
	StateIdle       State = "desconectado"
	StateConnecting State = "conectando"
	StateConnected  State = "no ar"
	StateRouteError State = "erro de rota"
	StateTunnelDown State = "túnel caiu"
)

type Status struct {
	State          State
	Gateway        string
	SystemIP       string
	DiscordIP      string
	GatewayBefore  string
	GatewayAfter   string
	RedirectBytes  uint64
	LogDir         string
	Error          string
}

type Session struct {
	mu sync.Mutex

	job        *job.Object
	vpn        vpn.Session
	redirector *exec.Cmd

	routeBefore route.Snapshot
	gatewayHost string
	logDir      string
	settings    config.Settings
	onUpdate    func(Status)
}

func NewSession(settings config.Settings, onUpdate func(Status)) *Session {
	return &Session{settings: settings, onUpdate: onUpdate}
}

func (s *Session) notify(st Status) {
	if s.onUpdate != nil {
		s.onUpdate(st)
	}
}

func (s *Session) Connect() {
	s.mu.Lock()
	if s.vpn.Running() {
		s.mu.Unlock()
		return
	}
	s.mu.Unlock() // long connect must not hold the lock

	s.notify(Status{State: StateConnecting})

	logDir, err := config.LogDir()
	if err != nil {
		s.notify(Status{State: StateIdle, Error: err.Error()})
		return
	}
	s.logDir = logDir

	if err := config.WriteRedirectorConfig(s.settings.Channels); err != nil {
		s.notify(Status{State: StateIdle, Error: err.Error()})
		return
	}

	before, err := route.Capture()
	if err != nil {
		s.notify(Status{State: StateIdle, Error: err.Error()})
		return
	}
	s.routeBefore = before

	j, err := job.NewKillOnClose()
	if err != nil {
		s.notify(Status{State: StateIdle, Error: err.Error()})
		return
	}
	s.job = j

	redCmd, err := redirect.Start()
	if err != nil {
		s.job.Close()
		s.notify(Status{State: StateIdle, Error: err.Error()})
		return
	}
	_ = s.job.Assign(redCmd.Process.Pid)
	s.redirector = redCmd

	runtimeDir, err := config.RuntimeDir()
	if err != nil {
		s.disconnectLocked()
		s.notify(Status{State: StateIdle, Error: err.Error()})
		return
	}

	client, err := riseup.NewClient()
	if err != nil {
		s.disconnectLocked()
		s.notify(Status{State: StateIdle, Error: err.Error()})
		return
	}
	certPEM, err := client.FetchCertificate()
	if err != nil {
		s.disconnectLocked()
		s.notify(Status{State: StateIdle, Error: err.Error()})
		return
	}
	gw, ovpnLines, err := client.PickGateway(s.settings.CountryCodes)
	if err != nil {
		s.disconnectLocked()
		s.notify(Status{State: StateIdle, Error: err.Error()})
		return
	}
	s.gatewayHost = fmt.Sprintf("%s (%s)", gw.Location, gw.Country)

	caPath := filepath.Join(runtimeDir, "ca.pem")
	certPath := filepath.Join(runtimeDir, "cert.pem")
	if err := os.WriteFile(caPath, riseup.CAPEM(), 0o600); err != nil {
		s.disconnectLocked()
		s.notify(Status{State: StateIdle, Error: err.Error()})
		return
	}
	if err := os.WriteFile(certPath, certPEM, 0o600); err != nil {
		s.disconnectLocked()
		s.notify(Status{State: StateIdle, Error: err.Error()})
		return
	}

	cfgPath, err := vpn.WriteConfig(runtimeDir, caPath, certPath, ovpnLines)
	if err != nil {
		s.disconnectLocked()
		s.notify(Status{State: StateIdle, Error: err.Error()})
		return
	}

	ovpnExe, err := vpn.FindOpenVPN()
	if err != nil {
		s.disconnectLocked()
		s.notify(Status{State: StateIdle, Error: err.Error()})
		return
	}
	logPath := filepath.Join(logDir, "openvpn.log")
	if _, err := s.vpn.Start(ovpnExe, cfgPath, logPath); err != nil {
		s.disconnectLocked()
		s.notify(Status{State: StateIdle, Error: err.Error()})
		return
	}
	if pid := s.vpn.PID(); pid != 0 {
		_ = s.job.Assign(pid)
	}

	if err := s.vpn.WaitConnected(90 * time.Second); err != nil {
		s.disconnectLocked()
		s.notify(Status{State: StateIdle, Error: err.Error()})
		return
	}

	after, err := route.Capture()
	if err != nil {
		s.disconnectLocked()
		s.notify(Status{State: StateRouteError, Error: err.Error()})
		return
	}
	if err := route.Validate(s.routeBefore, after); err != nil {
		s.disconnectLocked()
		route.CleanupSplitRoutes()
		s.notify(Status{State: StateRouteError, Error: err.Error()})
		return
	}

	sysIP, _ := egress.PublicIP()

	s.notify(Status{
		State:         StateConnected,
		Gateway:       s.gatewayHost,
		SystemIP:      sysIP,
		DiscordIP:     sysIP + " (via ponte quando Discord enviar tráfego)",
		GatewayBefore: s.routeBefore.DefaultGateway,
		GatewayAfter:  after.DefaultGateway,
		LogDir:        logDir,
	})
}

func (s *Session) Disconnect() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.disconnectLocked()
	route.CleanupSplitRoutes()
	s.notify(Status{State: StateIdle, LogDir: s.logDir})
}

func (s *Session) disconnectLocked() {
	s.vpn.Stop()
	if s.redirector != nil && s.redirector.Process != nil {
		_ = s.redirector.Process.Kill()
		s.redirector = nil
	}
	if s.job != nil {
		s.job.Close()
		s.job = nil
	}
	runtimeDir, err := config.RuntimeDir()
	if err == nil {
		_ = os.Remove(filepath.Join(runtimeDir, "cert.pem"))
	}
}
