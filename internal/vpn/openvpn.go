//go:build windows

package vpn

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type Session struct {
	mu        sync.Mutex
	cmd       *exec.Cmd
	mgmtPort  int
	logPath   string
	configPath string
}

func FindOpenVPN() (string, error) {
	candidates := []string{
		filepath.Join(os.Getenv("ProgramFiles"), "OpenVPN", "bin", "openvpn.exe"),
		filepath.Join(os.Getenv("ProgramFiles(x86)"), "OpenVPN", "bin", "openvpn.exe"),
		filepath.Join(os.Getenv("ProgramFiles"), "RiseupVPN", "openvpn.exe"),
	}
	for _, c := range candidates {
		if c != "" {
			if st, err := os.Stat(c); err == nil && !st.IsDir() {
				return c, nil
			}
		}
	}
	return "", fmt.Errorf("openvpn.exe não encontrado — instale o OpenVPN Community (openvpn.net) ou a RiseupVPN")
}

func WriteConfig(dir, caPath, certPath string, serverLines []string) (string, error) {
	var b strings.Builder
	b.WriteString("client\n")
	b.WriteString("dev tun\n")
	b.WriteString("nobind\n")
	b.WriteString("persist-tun\n")
	b.WriteString("tls-client\n")
	b.WriteString("remote-cert-tls server\n")
	b.WriteString("tls-version-min 1.0\n")
	b.WriteString("proto udp\n")
	b.WriteString("script-security 1\n")
	b.WriteString(fmt.Sprintf("ca %s\n", caPath))
	b.WriteString(fmt.Sprintf("cert %s\n", certPath))
	b.WriteString(fmt.Sprintf("key %s\n", certPath))
	for _, line := range serverLines {
		if strings.HasPrefix(line, "remote ") {
			b.WriteString(line + "\n")
		} else {
			b.WriteString(line + "\n")
		}
	}
	path := filepath.Join(dir, "session.ovpn")
	if err := os.WriteFile(path, []byte(b.String()), 0o600); err != nil {
		return "", err
	}
	return path, nil
}

func freePort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}

func (s *Session) Start(openvpnExe, configPath, logPath string) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cmd != nil && s.cmd.Process != nil {
		return s.mgmtPort, nil
	}
	port, err := freePort()
	if err != nil {
		return 0, err
	}
	args := []string{
		"--config", configPath,
		"--management", "127.0.0.1", fmt.Sprintf("%d", port),
		"--management-query-passwords", "once",
		"--windows-driver", "wintun",
		"--route-nopull",
		"--pull-filter", "ignore", "redirect-gateway",
		"--pull-filter", "ignore", "dhcp-option DNS",
		"--pull-filter", "ignore", "block-outside-dns",
		"--pull-filter", "ignore", "route-ipv6",
		"--pull-filter", "ignore", "ifconfig-ipv6",
		"--log", logPath,
		"--verb", "3",
	}
	cmd := exec.Command(openvpnExe, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return 0, err
	}
	s.cmd = cmd
	s.mgmtPort = port
	s.logPath = logPath
	s.configPath = configPath
	return port, nil
}

func (s *Session) WaitConnected(timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", s.mgmtPort), time.Second)
		if err != nil {
			time.Sleep(300 * time.Millisecond)
			continue
		}
		_ = conn.SetDeadline(time.Now().Add(2 * time.Second))
		r := bufio.NewReader(conn)
		for {
			line, err := r.ReadString('\n')
			if err != nil {
				break
			}
			if strings.HasPrefix(line, ">HOLD:") {
				_, _ = fmt.Fprint(conn, "hold release\n")
				continue
			}
			if strings.Contains(line, "CONNECTED,SUCCESS") {
				conn.Close()
				return nil
			}
			if strings.Contains(line, "FATAL") || strings.Contains(line, "AUTH_FAILED") {
				conn.Close()
				return fmt.Errorf("openvpn: %s", strings.TrimSpace(line))
			}
		}
		conn.Close()
		time.Sleep(400 * time.Millisecond)
	}
	return fmt.Errorf("openvpn: timeout aguardando CONNECTED")
}

func (s *Session) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cmd == nil || s.cmd.Process == nil {
		return
	}
	if s.mgmtPort != 0 {
		conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", s.mgmtPort), time.Second)
		if err == nil {
			fmt.Fprint(conn, "signal SIGTERM\n")
			conn.Close()
			time.Sleep(1500 * time.Millisecond)
		}
	}
	if s.cmd.ProcessState == nil {
		_ = s.cmd.Process.Kill()
	}
	s.cmd = nil
	s.mgmtPort = 0
}

func (s *Session) PID() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cmd == nil || s.cmd.Process == nil {
		return 0
	}
	return s.cmd.Process.Pid
}

func (s *Session) Running() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.cmd != nil && s.cmd.Process != nil && s.cmd.ProcessState == nil
}
