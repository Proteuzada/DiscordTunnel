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
	"syscall"
	"time"
	"unsafe"
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
	b.WriteString("script-security 1\n")
	b.WriteString("connect-retry-max 2\n")
	b.WriteString(fmt.Sprintf("ca %s\n", ovpnPath(caPath)))
	b.WriteString(fmt.Sprintf("cert %s\n", ovpnPath(certPath)))
	b.WriteString(fmt.Sprintf("key %s\n", ovpnPath(certPath)))
	hasRemote := false
	for _, line := range serverLines {
		if strings.HasPrefix(line, "remote ") {
			hasRemote = true
		}
		b.WriteString(line + "\n")
	}
	if hasRemote {
		b.WriteString("remote-random\n")
		b.WriteString("connect-timeout 20\n")
		b.WriteString("proto udp\n")
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
	_ = os.Remove(logPath)
	args := []string{
		"--config", configPath,
		"--management", "127.0.0.1", fmt.Sprintf("%d", port),
		// TAP evita conflito quando Wintun já está em uso (ex.: Cloudflare WARP).
		"--windows-driver", "tap-windows6",
		"--route-nopull",
		"--pull-filter", "ignore", "redirect-gateway",
		"--pull-filter", "ignore", "dhcp-option DNS",
		"--pull-filter", "ignore", "block-outside-dns",
		"--pull-filter", "ignore", "route-ipv6",
		"--pull-filter", "ignore", "ifconfig-ipv6",
		"--log", ovpnPath(logPath),
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
	addr := fmt.Sprintf("127.0.0.1:%d", s.mgmtPort)

	_ = addr
	for time.Now().Before(deadline) {
		if !processAlive(s.cmd) {
			if msg := tailLog(s.logPath); msg != "" {
				return fmt.Errorf("openvpn encerrou: %s", msg)
			}
			return fmt.Errorf("openvpn encerrou antes de conectar")
		}
		logText := readLogTail(s.logPath)
		if strings.Contains(logText, "Initialization Sequence Completed") {
			return nil
		}
		if strings.Contains(logText, "AUTH_FAILED") || strings.Contains(logText, "Exiting due to fatal error") {
			if msg := tailLog(s.logPath); msg != "" {
				return fmt.Errorf("%s", msg)
			}
			return fmt.Errorf("openvpn falhou ao conectar")
		}
		time.Sleep(300 * time.Millisecond)
	}
	if msg := tailLog(s.logPath); msg != "" {
		return fmt.Errorf("openvpn: timeout (%s)", msg)
	}
	return fmt.Errorf("openvpn: timeout aguardando CONNECTED")
}

func waitMgmtConnected(conn net.Conn, deadline time.Time) error {
	// OpenVPN 2.6 with --management-hold does not send packets until this.
	// Sending it up front matches the official client's management session.
	if _, err := fmt.Fprint(conn, "hold release\nstate on\n"); err != nil {
		return err
	}
	r := bufio.NewReader(conn)
	for time.Now().Before(deadline) {
		_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
		line, err := r.ReadString('\n')
		if err != nil {
			if ne, ok := err.(net.Error); ok && ne.Timeout() {
				continue
			}
			return err
		}
		line = strings.TrimSpace(line)
		if strings.Contains(line, "CONNECTED,SUCCESS") {
			return nil
		}
		if strings.Contains(line, "FATAL") || strings.Contains(line, "AUTH_FAILED") {
			return fmt.Errorf("%s", line)
		}
	}
	return fmt.Errorf("prazo esgotado")
}

func processAlive(cmd *exec.Cmd) bool {
	if cmd == nil || cmd.Process == nil {
		return false
	}
	const queryLimited = 0x1000
	const stillActive = 259
	h, err := syscall.OpenProcess(queryLimited, false, uint32(cmd.Process.Pid))
	if err != nil {
		return false
	}
	defer syscall.CloseHandle(h)
	var code uint32
	r, _, _ := syscall.NewLazyDLL("kernel32.dll").NewProc("GetExitCodeProcess").Call(
		uintptr(h),
		uintptr(unsafe.Pointer(&code)),
	)
	return r != 0 && code == stillActive
}

func PushedGateway(logPath string) string {
	text := readLogTail(logPath)
	const key = "route-gateway "
	last := ""
	for _, line := range strings.Split(text, "\n") {
		if i := strings.Index(line, key); i >= 0 {
			rest := strings.TrimSpace(line[i+len(key):])
			fields := strings.FieldsFunc(rest, func(r rune) bool { return r == ',' || r == ' ' })
			if len(fields) > 0 {
				last = fields[0]
			}
		}
	}
	return last
}

func ovpnPath(p string) string {
	return filepath.ToSlash(filepath.Clean(p))
}

func readLogTail(path string) string {
	if path == "" {
		return ""
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	s := string(b)
	if len(s) > 16000 {
		s = s[len(s)-16000:]
	}
	return s
}

func tailLog(path string) string {
	s := readLogTail(path)
	if s == "" {
		return ""
	}
	if len(s) > 800 {
		s = s[len(s)-800:]
	}
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.Contains(line, "wintun adapters") && strings.Contains(line, "in use") {
			return "Adaptador Wintun ocupado (feche Cloudflare WARP ou outra VPN) e tente de novo"
		}
		if strings.Contains(strings.ToUpper(line), "ERROR") ||
			strings.Contains(strings.ToUpper(line), "FATAL") ||
			strings.Contains(line, "Exiting") {
			return line
		}
	}
	if strings.Contains(s, "Initialization Sequence Completed") {
		return ""
	}
	return strings.TrimSpace(lastLine(s))
}

func lastLine(s string) string {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == '\n' {
			return strings.TrimSpace(s[i+1:])
		}
	}
	return strings.TrimSpace(s)
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
