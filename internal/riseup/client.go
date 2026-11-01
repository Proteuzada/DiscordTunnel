package riseup

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
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
	"auth": true, "cipher": true, "keepalive": true, "tun-ipv6": true, "float": true,
}

type Client struct {
	http *http.Client
}

func NewClient() (*Client, error) {
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(riseupCA) {
		return nil, fmt.Errorf("riseup: CA inválido")
	}
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{
			RootCAs:    pool,
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
				Type  string `json:"type"`
				Ports []int  `json:"ports"`
			} `json:"transport"`
		} `json:"capabilities"`
	} `json:"gateways"`
	Locations map[string]struct {
		CountryCode string `json:"country_code"`
		City        string `json:"city"`
	} `json:"locations"`
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
			var ports []int
			for _, p := range tp.Ports {
				if p == 53 {
					continue
				}
				ports = append(ports, p)
			}
			if len(ports) == 0 {
				continue
			}
			city := loc.City
			if city == "" {
				city = gw.Location
			}
			candidates = append(candidates, Gateway{
				Host:     gw.IPAddress,
				Ports:    ports,
				Location: city,
				Country:  loc.CountryCode,
			})
		}
	}
	if len(candidates) == 0 {
		return Gateway{}, nil, fmt.Errorf("riseup: nenhum gateway OpenVPN nos países %v", countryCodes)
	}
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].Host < candidates[j].Host
	})
	gw := candidates[0]

	lines := buildOpenVPNBase(cfg.OpenVPNConfiguration)
	for _, port := range gw.Ports {
		lines = append(lines, fmt.Sprintf("remote %s %d udp", gw.Host, port))
	}
	return gw, lines, nil
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
		var s string
		if err := json.Unmarshal(val, &s); err == nil && s != "" {
			lines = append(lines, opt+" "+s)
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
