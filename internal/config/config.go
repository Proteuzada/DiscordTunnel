package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type Channels struct {
	Stable      bool `json:"stable"`
	Canary      bool `json:"canary"`
	PTB         bool `json:"ptb"`
	Development bool `json:"development"`
}

type Settings struct {
	Channels     Channels `json:"channels"`
	CountryCodes []string `json:"countryCodes"`
}

func DefaultSettings() Settings {
	return Settings{
		Channels: Channels{Stable: true, Canary: true, PTB: true, Development: true},
		CountryCodes: []string{"BR", "US"},
	}
}

func AppDir() (string, error) {
	local := os.Getenv("LOCALAPPDATA")
	if local == "" {
		return "", os.ErrNotExist
	}
	dir := filepath.Join(local, "DiscordTunnel")
	return dir, os.MkdirAll(dir, 0o700)
}

func RuntimeDir() (string, error) {
	base, err := AppDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(base, "runtime")
	return dir, os.MkdirAll(dir, 0o700)
}

func LogDir() (string, error) {
	base, err := AppDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(base, "logs")
	return dir, os.MkdirAll(dir, 0o700)
}

func SettingsPath() (string, error) {
	base, err := AppDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "settings.json"), nil
}

func LoadSettings() Settings {
	path, err := SettingsPath()
	if err != nil {
		return DefaultSettings()
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return DefaultSettings()
	}
	var s Settings
	if json.Unmarshal(b, &s) != nil {
		return DefaultSettings()
	}
	if len(s.CountryCodes) == 0 {
		s.CountryCodes = DefaultSettings().CountryCodes
	}
	return s
}

func SaveSettings(s Settings) error {
	path, err := SettingsPath()
	if err != nil {
		return err
	}
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o600)
}

type RedirectorConfig struct {
	Prefixes     []string `json:"prefixes"`
	ExcludeNames []string `json:"excludeNames"`
}

func DiscordInstallPrefixes(ch Channels) []string {
	local := os.Getenv("LOCALAPPDATA")
	if local == "" {
		return nil
	}
	type pair struct {
		on   bool
		name string
	}
	pairs := []pair{
		{ch.Stable, "Discord"},
		{ch.Canary, "DiscordCanary"},
		{ch.PTB, "DiscordPTB"},
		{ch.Development, "DiscordDevelopment"},
	}
	var out []string
	for _, p := range pairs {
		if !p.on {
			continue
		}
		out = append(out, filepath.Join(local, p.name)+string(os.PathSeparator))
	}
	return out
}

func WriteRedirectorConfig(ch Channels) error {
	base, err := AppDir()
	if err != nil {
		return err
	}
	cfg := RedirectorConfig{
		Prefixes:     DiscordInstallPrefixes(ch),
		ExcludeNames: []string{"Update.exe"},
	}
	b, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(base, "redirector.json"), b, 0o600)
}
