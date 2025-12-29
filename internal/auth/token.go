package auth

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"
)

type Token struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	TokenType    string    `json:"token_type"`
	Expiry       time.Time `json:"expiry"`
}

func TokenPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(dir, "hidrive-cli", "token.json"), nil
}

func LoadToken() (Token, error) {
	var token Token
	path, err := TokenPath()
	if err != nil {
		return token, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return token, err
	}

	if err := json.Unmarshal(data, &token); err != nil {
		return token, err
	}

	return token, nil
}

func SaveToken(token Token) error {
	path, err := TokenPath()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}

	data, err := json.MarshalIndent(token, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0o600)
}

func HasValidToken(token Token, now time.Time) error {
	if token.AccessToken == "" {
		return errors.New("missing access token")
	}
	if token.Expiry.IsZero() {
		return errors.New("missing token expiry")
	}
	if now.After(token.Expiry) {
		return errors.New("token expired")
	}
	return nil
}
