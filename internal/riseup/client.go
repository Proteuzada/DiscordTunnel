package riseup

import (
	"bytes"
	"crypto/tls"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"
)

//go:embed riseup-ca.crt
var riseupCA []byte

const apiBase = "https://api.black.riseup.net/3/"

var allowedOpenVPNOptions = map[string]bool{
	"auth": true, "cipher": true, "data-ciphers": true, "keepalive": true,
	"tun-ipv6": true, "float": true, "tls-cipher": true, "tls-version-min": true,
	"persist-key": true, "rcvbuf": true, "sndbuf": true,
}

type Client struct {
	http *http.Client
}

func NewClient() (*Client, error) {
	// api.black.riseup.net is issued by a public CA (e.g. Let's Encrypt).
	// riseup-ca.crt is for OpenVPN only, not this HTTPS endpoint.
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{
			MinVersion: tls.VersionTLS12,
		},
	}
	return &Client{http: &http.Client{Timeout: 45 * time.Second, Transport: tr}}, nil
}

type Gateway struct {
	Host     string
	Ports    []int
	Location string
	Country  string
}

func (c *Client) FetchCertificate() ([]byte, error) {
	return c.postRaw("cert")
}

type eipConfig struct {
	OpenVPNConfiguration map[string]json.RawMessage `json:"openvpn_configuration"`
	Gateways             []struct {
		IPAddress string `json:"ip_address"`
		Location  string `json:"location"`
		Capabilities struct {
			Transport []struct {
				Type  string   `json:"type"`
				Ports []string `json:"ports"`
			} `json:"transport"`
		} `json:"capabilities"`
	} `json:"gateways"`
	Locations map[string]struct {
		CountryCode string `json:"country_code"`
		Name        string `json:"name"`
		City        string `json:"city"`
	} `json:"locations"`
}

func parseOpenVPNPorts(raw []string) []int {
	var out []int
	for _, s := range raw {
		if s == "53" || s == "80" {
			continue
		}
		var p int
		if _, err := fmt.Sscanf(s, "%d", &p); err != nil || p <= 0 {
			continue
		}
		out = append(out, p)
	}
	return out
}

func preferredOpenVPNPort(ports []int) int {
	for _, p := range ports {
		if p == 1194 {
			return 1194
		}
	}
	if len(ports) > 0 {
		return ports[0]
	}
	return 0
}

func (c *Client) PickGateway(countryCodes []string) (Gateway, []string, error) {
	body, err := c.postRaw("config/eip-service.json")
	if err != nil {
		return Gateway{}, nil, err
	}
	var cfg eipConfig
	if err := json.Unmarshal(body, &cfg); err != nil {
		return Gateway{}, nil, err
	}

	want := make(map[string]bool, len(countryCodes))
	for _, cc := range countryCodes {
		want[strings.ToUpper(strings.TrimSpace(cc))] = true
	}

	var candidates []Gateway
	for _, gw := range cfg.Gateways {
		loc, ok := cfg.Locations[gw.Location]
		if !ok {
			continue
		}
		if len(want) > 0 && !want[strings.ToUpper(loc.CountryCode)] {
			continue
		}
		for _, tp := range gw.Capabilities.Transport {
			if tp.Type != "openvpn" {
				continue
			}
			ports := parseOpenVPNPorts(tp.Ports)
			if len(ports) == 0 {
				continue
			}
			city := loc.City
			if city == "" {
				city = loc.Name
			}
			if city == "" {
				city = gw.Location
			}
			port := preferredOpenVPNPort(ports)
			if port == 0 {
				continue
			}
			candidates = append(candidates, Gateway{
				Host:     gw.IPAddress,
				Ports:    []int{port},
				Location: city,
				Country:  loc.CountryCode,
			})
		}
	}
	if len(candidates) == 0 && len(want) > 0 {
		// Retry without country filter (same as accepting any Riseup gateway).
		for _, gw := range cfg.Gateways {
			loc, ok := cfg.Locations[gw.Location]
			if !ok {
				continue
			}
			for _, tp := range gw.Capabilities.Transport {
				if tp.Type != "openvpn" {
					continue
				}
				ports := parseOpenVPNPorts(tp.Ports)
				if len(ports) == 0 {
					continue
				}
				city := loc.City
				if city == "" {
					city = loc.Name
				}
				if city == "" {
					city = gw.Location
				}
				port := preferredOpenVPNPort(ports)
				if port == 0 {
					continue
				}
				candidates = append(candidates, Gateway{
					Host:     gw.IPAddress,
					Ports:    []int{port},
					Location: city,
					Country:  loc.CountryCode,
				})
			}
		}
	}
	if len(candidates) == 0 {
		return Gateway{}, nil, fmt.Errorf("riseup: nenhum gateway OpenVPN disponível")
	}
	sort.Slice(candidates, func(i, j int) bool {
		pi, pj := gatewayPriority(candidates[i].Country), gatewayPriority(candidates[j].Country)
		if pi != pj {
			return pi < pj
		}
		return candidates[i].Host < candidates[j].Host
	})
	gw := candidates[0]

	lines := buildOpenVPNBase(cfg.OpenVPNConfiguration)
	limit := len(candidates)
	if limit > 6 {
		limit = 6
	}
	for i := 0; i < limit; i++ {
		g := candidates[i]
		for _, port := range g.Ports {
			lines = append(lines, fmt.Sprintf("remote %s %d", g.Host, port))
		}
	}
	return gw, lines, nil
}

func gatewayPriority(country string) int {
	switch strings.ToUpper(country) {
	case "BR", "FR", "NL", "DE", "ES", "IT", "PT":
		return 0
	case "US", "CA":
		return 2
	default:
		return 1
	}
}

func buildOpenVPNBase(opts map[string]json.RawMessage) []string {
	var lines []string
	keys := make([]string, 0, len(opts))
	for k := range opts {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, opt := range keys {
		if !allowedOpenVPNOptions[opt] {
			continue
		}
		val := opts[opt]
		if string(val) == "true" || string(val) == "null" {
			lines = append(lines, opt)
			continue
		}
		if string(val) == `""` || string(val) == "false" {
			if opt == "float" {
				lines = append(lines, opt)
			}
			continue
		}
		var s string
		if err := json.Unmarshal(val, &s); err == nil && s != "" {
			lines = append(lines, opt+" "+s)
			continue
		}
		var n json.Number
		if err := json.Unmarshal(val, &n); err == nil {
			lines = append(lines, opt+" "+n.String())
		}
	}
	return lines
}

func (c *Client) postRaw(path string) ([]byte, error) {
	req, err := http.NewRequest(http.MethodPost, apiBase+path, bytes.NewReader(nil))
	if err != nil {
		return nil, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("riseup: %s -> HTTP %d: %s", path, resp.StatusCode, strings.TrimSpace(string(b)))
	}
	return io.ReadAll(resp.Body)
}
