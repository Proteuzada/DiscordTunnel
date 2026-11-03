package update

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/mello/tunneling/internal/version"
	"github.com/mello/tunneling/internal/winexec"
)

const latestURL = "https://api.github.com/repos/Proteuzada/DiscordTunnel/releases/latest"

type Release struct {
	Version string
	URL     string
}

type ghRelease struct {
	TagName string `json:"tag_name"`
	Assets  []struct {
		Name string `json:"name"`
		URL  string `json:"browser_download_url"`
	} `json:"assets"`
}

// NewerRelease returns the latest GitHub installer when it is newer than this build.
func NewerRelease() (*Release, error) {
	req, err := http.NewRequest(http.MethodGet, latestURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "DiscordTunnel/"+version.Current)

	client := &http.Client{Timeout: 20 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("github: HTTP %d", resp.StatusCode)
	}
	var rel ghRelease
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return nil, err
	}
	tag := strings.TrimPrefix(strings.TrimSpace(rel.TagName), "v")
	if tag == "" || !lessVersion(version.Current, tag) {
		return nil, nil
	}
	for _, a := range rel.Assets {
		name := strings.ToLower(a.Name)
		if strings.HasPrefix(name, "discordtunnel-setup-") && strings.HasSuffix(name, ".exe") {
			return &Release{Version: tag, URL: a.URL}, nil
		}
	}
	return nil, fmt.Errorf("github: release %s sem instalador", tag)
}

// Install downloads the setup and starts it silently. The caller should exit
// so the installer can replace DiscordTunnel.exe.
func Install(rel *Release) error {
	if rel == nil || rel.URL == "" {
		return fmt.Errorf("atualização sem endereço")
	}
	client := &http.Client{Timeout: 3 * time.Minute}
	resp, err := client.Get(rel.URL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download: HTTP %d", resp.StatusCode)
	}
	dest := filepath.Join(os.TempDir(), "DiscordTunnel-Setup-"+rel.Version+".exe")
	f, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o700)
	if err != nil {
		return err
	}
	if _, err := io.Copy(f, resp.Body); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	cmd := winexec.Command(dest, "/VERYSILENT", "/SUPPRESSMSGBOXES", "/NORESTART", "/CLOSEAPPLICATIONS")
	return cmd.Start()
}

func lessVersion(current, next string) bool {
	a := parseVersion(current)
	b := parseVersion(next)
	for i := 0; i < len(a) || i < len(b); i++ {
		av, bv := 0, 0
		if i < len(a) {
			av = a[i]
		}
		if i < len(b) {
			bv = b[i]
		}
		if av < bv {
			return true
		}
		if av > bv {
			return false
		}
	}
	return false
}

func parseVersion(s string) []int {
	s = strings.TrimPrefix(strings.TrimSpace(s), "v")
	parts := strings.Split(s, ".")
	out := make([]int, 0, len(parts))
	for _, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil {
			n = 0
		}
		out = append(out, n)
	}
	return out
}
