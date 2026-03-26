package openaiimport

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"
)

const (
	defaultRedirectURL = "http://localhost:1455/auth/callback"
	platformOpenAI     = "openai"
	platformSora       = "sora"
)

type Config struct {
	ServerURL   string
	APIKey      string
	Platform    string
	Name        string
	ProxyID     *int64
	Concurrency int
	Priority    int
	GroupIDs    []int64
	RedirectURL string
	NoOpen      bool
	StatusOut   io.Writer
}

func (c Config) Validate() error {
	if strings.TrimSpace(c.ServerURL) == "" {
		return fmt.Errorf("server url is required")
	}
	if strings.TrimSpace(c.APIKey) == "" {
		return fmt.Errorf("api key is required")
	}
	if c.Concurrency <= 0 {
		return fmt.Errorf("concurrency must be greater than 0")
	}
	if strings.TrimSpace(c.RedirectURL) == "" {
		return fmt.Errorf("redirect url is required")
	}
	switch normalizePlatform(c.Platform) {
	case platformOpenAI, platformSora:
		return nil
	default:
		return fmt.Errorf("unsupported platform %q", c.Platform)
	}
}

type Result struct {
	Account  *account `json:"account"`
	AuthURL  string   `json:"auth_url"`
	Platform string   `json:"platform"`
}

func Run(ctx context.Context, cfg Config) (*Result, error) {
	if cfg.RedirectURL == "" {
		cfg.RedirectURL = defaultRedirectURL
	}
	if cfg.StatusOut == nil {
		cfg.StatusOut = io.Discard
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	client, err := newClient(cfg.ServerURL, cfg.APIKey, cfg.Platform)
	if err != nil {
		return nil, err
	}

	callback, err := newCallbackServer(cfg.RedirectURL)
	if err != nil {
		return nil, err
	}
	if err := callback.start(); err != nil {
		return nil, fmt.Errorf("start callback server: %w", err)
	}
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = callback.shutdown(shutdownCtx)
	}()

	fmt.Fprintf(cfg.StatusOut, "Listening for OAuth callback on %s\n", cfg.RedirectURL)

	auth, err := client.generateAuthURL(ctx, generateAuthURLRequest{
		RedirectURI: cfg.RedirectURL,
		ProxyID:     cfg.ProxyID,
	})
	if err != nil {
		return nil, err
	}

	printAuthInstructions(cfg.StatusOut, auth.AuthURL, cfg.NoOpen)
	if !cfg.NoOpen {
		if err := openBrowser(auth.AuthURL); err != nil {
			fmt.Fprintf(cfg.StatusOut, "Failed to open browser automatically: %v\n", err)
			fmt.Fprintln(cfg.StatusOut, "Open the URL above manually.")
		}
	}

	callbackResult, err := callback.wait(ctx)
	if err != nil {
		return nil, fmt.Errorf("wait for oauth callback: %w", err)
	}
	if callbackResult.Error != "" {
		return nil, fmt.Errorf("oauth callback returned error: %s", callbackResult.errorMessage())
	}

	fmt.Fprintln(cfg.StatusOut, "OAuth callback received. Creating account...")

	acc, err := client.createAccountFromOAuth(ctx, createAccountFromOAuthRequest{
		SessionID:   auth.SessionID,
		Code:        callbackResult.Code,
		State:       callbackResult.State,
		RedirectURI: cfg.RedirectURL,
		ProxyID:     cfg.ProxyID,
		Name:        strings.TrimSpace(cfg.Name),
		Concurrency: cfg.Concurrency,
		Priority:    cfg.Priority,
		GroupIDs:    cfg.GroupIDs,
	})
	if err != nil {
		return nil, err
	}

	fmt.Fprintf(cfg.StatusOut, "Account created: id=%d name=%s platform=%s\n", acc.ID, acc.Name, acc.Platform)

	return &Result{
		Account:  acc,
		AuthURL:  auth.AuthURL,
		Platform: normalizePlatform(cfg.Platform),
	}, nil
}

func printAuthInstructions(w io.Writer, authURL string, noOpen bool) {
	if noOpen {
		fmt.Fprintln(w, "Open this URL in a browser to continue:")
	} else {
		fmt.Fprintln(w, "Opening browser for OAuth authorization...")
		fmt.Fprintln(w, "If your browser does not open, use this URL:")
	}
	fmt.Fprintln(w, authURL)
}

func normalizePlatform(platform string) string {
	switch strings.ToLower(strings.TrimSpace(platform)) {
	case platformSora:
		return platformSora
	default:
		return platformOpenAI
	}
}

func (r Result) MarshalJSON() ([]byte, error) {
	type alias Result
	return json.Marshal(alias(r))
}
