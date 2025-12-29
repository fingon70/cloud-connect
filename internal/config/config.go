package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
)

const DefaultRedirectURI = "http://localhost:8888/callback"

type Config struct {
	ClientID     string   `json:"client_id"`
	ClientSecret string   `json:"client_secret"`
	RedirectURI  string   `json:"redirect_uri"`
	Scopes       []string `json:"scopes"`
}

func DefaultConfigPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(dir, "hidrive-cli", "config.json"), nil
}

func Load() (Config, error) {
	cfg := Config{
		RedirectURI: DefaultRedirectURI,
		Scopes:      []string{"rw"},
	}

	path, err := DefaultConfigPath()
	if err != nil {
		return cfg, err
	}

	data, err := os.ReadFile(path)
	if err == nil {
		if err := json.Unmarshal(data, &cfg); err != nil {
			return cfg, err
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return cfg, err
	}

	applyEnv(&cfg)
	return cfg, nil
}

func applyEnv(cfg *Config) {
	if value, ok := os.LookupEnv("HIDRIVE_CLIENT_ID"); ok {
		cfg.ClientID = value
	}
	if value, ok := os.LookupEnv("HIDRIVE_CLIENT_SECRET"); ok {
		cfg.ClientSecret = value
	}
	if value, ok := os.LookupEnv("HIDRIVE_REDIRECT_URI"); ok {
		cfg.RedirectURI = value
	}
	if value, ok := os.LookupEnv("HIDRIVE_SCOPES"); ok {
		cfg.Scopes = splitCSV(value)
	}
}

func splitCSV(value string) []string {
	parts := strings.Split(value, ",")
	var out []string
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		out = append(out, part)
	}
	return out
}
