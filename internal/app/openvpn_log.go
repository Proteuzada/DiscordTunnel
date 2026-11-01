//go:build windows

package app

import (
	"os"
	"path/filepath"
	"strings"
)

func readOpenVPNHint(logDir string) string {
	if logDir == "" {
		return ""
	}
	b, err := os.ReadFile(filepath.Join(logDir, "openvpn.log"))
	if err != nil {
		return ""
	}
	s := string(b)
	if strings.Contains(s, "wintun adapters") && strings.Contains(s, "in use") {
		return "Wintun em uso — desative o Cloudflare WARP (ou outra VPN) e conecte de novo."
	}
	if strings.Contains(s, "All tap-windows6 adapters") && strings.Contains(s, "disabled") {
		return "Adaptador TAP indisponível — reinstale o OpenVPN com o driver TAP/Wintun."
	}
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if strings.Contains(line, "Exiting due to fatal error") {
			continue
		}
		if strings.Contains(strings.ToUpper(line), "FATAL") || strings.Contains(line, "Cannot") {
			return line
		}
	}
	return ""
}
