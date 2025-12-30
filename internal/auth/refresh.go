package auth

import (
	"context"
	"errors"

	"github.com/fingon70/cloud-connect/internal/config"
	"golang.org/x/oauth2"
)

var ErrMissingRefreshToken = errors.New("missing refresh token")

func Refresh(ctx context.Context, cfg config.Config, token Token) (Token, error) {
	if cfg.ClientID == "" || cfg.ClientSecret == "" {
		return Token{}, errors.New("missing HIDRIVE_CLIENT_ID or HIDRIVE_CLIENT_SECRET")
	}
	if token.RefreshToken == "" {
		return Token{}, ErrMissingRefreshToken
	}

	oauthCfg, err := oauthConfig(cfg)
	if err != nil {
		return Token{}, err
	}

	source := oauthCfg.TokenSource(ctx, &oauth2.Token{
		AccessToken:  token.AccessToken,
		RefreshToken: token.RefreshToken,
		TokenType:    token.TokenType,
		Expiry:       token.Expiry,
	})

	refreshed, err := source.Token()
	if err != nil {
		return Token{}, err
	}

	next := Token{
		AccessToken:  refreshed.AccessToken,
		RefreshToken: refreshed.RefreshToken,
		TokenType:    refreshed.TokenType,
		Expiry:       refreshed.Expiry,
	}
	if next.RefreshToken == "" {
		next.RefreshToken = token.RefreshToken
	}

	return next, nil
}
