//go:build windows

package egress

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"syscall"
	"time"
)

const checkURL = "https://api.ipify.org?format=json"

func TunnelIfIndex() (uint32, error) {
	ifs, err := net.Interfaces()
	if err != nil {
		return 0, err
	}
	for _, iface := range ifs {
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, a := range addrs {
			ipnet, ok := a.(*net.IPNet)
			if !ok {
				continue
			}
			ip := ipnet.IP.To4()
			if ip != nil && ip[0] == 10 && (ip[1] == 41 || ip[1] == 42) {
				return uint32(iface.Index), nil
			}
		}
	}
	return 0, fmt.Errorf("adaptador da Riseup ainda sem IPv4")
}

func TunnelPublicIP() (string, error) {
	idx, err := TunnelIfIndex()
	if err != nil {
		return "", err
	}
	return PublicIPViaInterface(idx)
}

func PublicIP() (string, error) {
	client := &http.Client{Timeout: 12 * time.Second}
	resp, err := client.Get(checkURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	return parseIP(resp.Body)
}

func PublicIPViaInterface(ifIndex uint32) (string, error) {
	d := &net.Dialer{
		Timeout: 12 * time.Second,
		Control: func(network, address string, c syscall.RawConn) error {
			return c.Control(func(fd uintptr) {
				_ = bindToInterface(fd, ifIndex)
			})
		},
	}
	client := &http.Client{
		Timeout: 12 * time.Second,
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				return d.DialContext(ctx, "tcp4", addr)
			},
		},
	}
	resp, err := client.Get(checkURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	return parseIP(resp.Body)
}

func bindToInterface(fd uintptr, ifIndex uint32) error {
	const ipUnicastIf = 31
	var idx uint32 = ifIndex
	return syscall.SetsockoptInt(syscall.Handle(fd), syscall.IPPROTO_IP, ipUnicastIf, int(idx))
}

func parseIP(r io.Reader) (string, error) {
	b, err := io.ReadAll(r)
	if err != nil {
		return "", err
	}
	return parseIPFromJSON(b)
}

func parseIPFromJSON(b []byte) (string, error) {
	var out struct {
		IP string `json:"ip"`
	}
	if err := json.Unmarshal(b, &out); err != nil {
		return "", err
	}
	if out.IP == "" {
		return "", fmt.Errorf("resposta vazia")
	}
	return out.IP, nil
}

func extractHTTPBody(raw []byte) []byte {
	s := string(raw)
	if i := findBodyStart(s); i >= 0 {
		return []byte(s[i:])
	}
	return raw
}

func findBodyStart(s string) int {
	idx := 0
	for {
		pos := indexOf(s[idx:], "\r\n\r\n")
		if pos < 0 {
			return -1
		}
		idx += pos + 4
		if idx < len(s) {
			return idx
		}
	}
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
