package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"time"

	"github.com/fingon70/cloud-connect/internal/config"
	"golang.org/x/oauth2"
)

const (
	authURL  = "https://my.hidrive.com/client/authorize"
	tokenURL = "https://my.hidrive.com/oauth2/token"
)

func Login(ctx context.Context, cfg config.Config) (Token, error) {
	var token Token
	if cfg.ClientID == "" || cfg.ClientSecret == "" {
		return token, errors.New("missing HIDRIVE_CLIENT_ID or HIDRIVE_CLIENT_SECRET")
	}

	oauthCfg, err := oauthConfig(cfg)
	if err != nil {
		return token, err
	}

	state, err := randomState()
	if err != nil {
		return token, err
	}

	codeCh := make(chan string, 1)
	errCh := make(chan error, 1)
	server, err := startCallbackServer(cfg.RedirectURI, state, codeCh, errCh)
	if err != nil {
		return token, err
	}
	defer shutdownServer(server)

	authURL := oauthCfg.AuthCodeURL(state, oauth2.AccessTypeOffline)
	if err := openBrowser(authURL); err != nil {
		fmt.Printf("Open this URL in your browser:\n%s\n", authURL)
	}

	select {
	case code := <-codeCh:
		oauthToken, err := oauthCfg.Exchange(ctx, code)
		if err != nil {
			return token, err
		}
		token = Token{
			AccessToken:  oauthToken.AccessToken,
			RefreshToken: oauthToken.RefreshToken,
			TokenType:    oauthToken.TokenType,
			Expiry:       oauthToken.Expiry,
		}
		return token, nil
	case err := <-errCh:
		return token, err
	case <-ctx.Done():
		return token, ctx.Err()
	}
}

func oauthConfig(cfg config.Config) (*oauth2.Config, error) {
	_, err := url.Parse(cfg.RedirectURI)
	if err != nil {
		return nil, fmt.Errorf("invalid redirect uri: %w", err)
	}

	return &oauth2.Config{
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		RedirectURL:  cfg.RedirectURI,
		Scopes:       cfg.Scopes,
		Endpoint: oauth2.Endpoint{
			AuthURL:  authURL,
			TokenURL: tokenURL,
		},
	}, nil
}

func startCallbackServer(redirectURI, expectedState string, codeCh chan<- string, errCh chan<- error) (*http.Server, error) {
	parsed, err := url.Parse(redirectURI)
	if err != nil {
		return nil, err
	}

	mux := http.NewServeMux()
	server := &http.Server{
		Addr:         parsed.Host,
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	mux.HandleFunc(parsed.Path, func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		state := query.Get("state")
		if state != expectedState {
			errCh <- errors.New("state mismatch in callback")
			http.Error(w, "state mismatch", http.StatusBadRequest)
			return
		}

		code := query.Get("code")
		if code == "" {
			errCh <- errors.New("missing code in callback")
			http.Error(w, "missing code", http.StatusBadRequest)
			return
		}

		fmt.Fprintln(w, "Login complete. You can close this window.")
		codeCh <- code
	})

	listener, err := net.Listen("tcp", parsed.Host)
	if err != nil {
		return nil, err
	}

	go func() {
		if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	return server, nil
}

func shutdownServer(server *http.Server) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = server.Shutdown(ctx)
}

func randomState() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}
