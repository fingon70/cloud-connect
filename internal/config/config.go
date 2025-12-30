package config

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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

func Save(cfg Config) error {
	path, err := DefaultConfigPath()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0o600)
}

func PromptMissing(cfg *Config, in io.Reader, out io.Writer) (bool, error) {
	changed := false
	reader := bufio.NewReader(in)

	if strings.TrimSpace(cfg.ClientID) == "" {
		value, err := promptRequired(reader, out, "HiDrive client ID")
		if err != nil {
			return false, err
		}
		cfg.ClientID = value
		changed = true
	}

	if strings.TrimSpace(cfg.ClientSecret) == "" {
		value, err := promptRequired(reader, out, "HiDrive client secret")
		if err != nil {
			return false, err
		}
		cfg.ClientSecret = value
		changed = true
	}

	return changed, nil
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

func promptRequired(reader *bufio.Reader, out io.Writer, label string) (string, error) {
	for {
		if _, err := fmt.Fprintf(out, "%s: ", label); err != nil {
			return "", err
		}
		line, err := reader.ReadString('\n')
		if err != nil && !errors.Is(err, io.EOF) {
			return "", err
		}
		value := strings.TrimSpace(line)
		if value != "" {
			return value, nil
		}
		if errors.Is(err, io.EOF) {
			return "", errors.New("input required")
		}
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
