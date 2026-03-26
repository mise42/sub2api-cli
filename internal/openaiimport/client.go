package openaiimport

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const defaultHTTPTimeout = 30 * time.Second

type client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
	platform   string
}

type generateAuthURLRequest struct {
	RedirectURI string `json:"redirect_uri,omitempty"`
	ProxyID     *int64 `json:"proxy_id,omitempty"`
}

type generateAuthURLResponse struct {
	AuthURL   string `json:"auth_url"`
	SessionID string `json:"session_id"`
}

type createAccountFromOAuthRequest struct {
	SessionID   string  `json:"session_id"`
	Code        string  `json:"code"`
	State       string  `json:"state"`
	RedirectURI string  `json:"redirect_uri,omitempty"`
	ProxyID     *int64  `json:"proxy_id,omitempty"`
	Name        string  `json:"name,omitempty"`
	Concurrency int     `json:"concurrency"`
	Priority    int     `json:"priority"`
	GroupIDs    []int64 `json:"group_ids,omitempty"`
}

type account struct {
	ID          int64   `json:"id"`
	Name        string  `json:"name"`
	Platform    string  `json:"platform"`
	Type        string  `json:"type"`
	Status      string  `json:"status"`
	ProxyID     *int64  `json:"proxy_id"`
	Concurrency int     `json:"concurrency"`
	Priority    int     `json:"priority"`
	GroupIDs    []int64 `json:"group_ids"`
}

type apiEnvelope[T any] struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    T      `json:"data"`
}

type apiErrorBody struct {
	Code    any    `json:"code"`
	Message string `json:"message"`
	Detail  string `json:"detail"`
	Error   string `json:"error"`
}

func newClient(baseURL, apiKey, platform string) (*client, error) {
	normalizedBaseURL, err := normalizeAPIBaseURL(baseURL)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(apiKey) == "" {
		return nil, fmt.Errorf("api key is required")
	}

	return &client{
		baseURL: normalizedBaseURL,
		apiKey:  strings.TrimSpace(apiKey),
		httpClient: &http.Client{
			Timeout: defaultHTTPTimeout,
		},
		platform: normalizePlatform(platform),
	}, nil
}

func (c *client) generateAuthURL(ctx context.Context, req generateAuthURLRequest) (*generateAuthURLResponse, error) {
	path := "/admin/openai/generate-auth-url"
	if c.platform == platformSora {
		path = "/admin/sora/generate-auth-url"
	}
	return doJSON[generateAuthURLResponse](ctx, c, http.MethodPost, path, req)
}

func (c *client) createAccountFromOAuth(ctx context.Context, req createAccountFromOAuthRequest) (*account, error) {
	path := "/admin/openai/create-from-oauth"
	if c.platform == platformSora {
		path = "/admin/sora/create-from-oauth"
	}
	return doJSON[account](ctx, c, http.MethodPost, path, req)
}

func doJSON[T any](ctx context.Context, c *client, method, path string, payload any) (*T, error) {
	var body io.Reader
	if payload != nil {
		raw, err := json.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("marshal %s %s payload: %w", method, path, err)
		}
		body = bytes.NewReader(raw)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
	if err != nil {
		return nil, fmt.Errorf("build %s %s request: %w", method, path, err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%s %s failed: %w", method, path, err)
	}
	defer resp.Body.Close()

	rawResp, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read %s %s response: %w", method, path, err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("%s %s: %s", method, path, formatAPIError(rawResp, resp.Status))
	}

	var envelope apiEnvelope[T]
	if err := json.Unmarshal(rawResp, &envelope); err != nil {
		return nil, fmt.Errorf("decode %s %s response: %w", method, path, err)
	}

	return &envelope.Data, nil
}

func formatAPIError(raw []byte, fallback string) string {
	if len(bytes.TrimSpace(raw)) == 0 {
		return fallback
	}

	var body apiErrorBody
	if err := json.Unmarshal(raw, &body); err != nil {
		text := strings.TrimSpace(string(raw))
		if text == "" {
			return fallback
		}
		return text
	}

	for _, candidate := range []string{body.Detail, body.Message, body.Error} {
		if strings.TrimSpace(candidate) != "" {
			return strings.TrimSpace(candidate)
		}
	}

	return fallback
}

func normalizeAPIBaseURL(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", fmt.Errorf("server url is required")
	}

	u, err := url.Parse(trimmed)
	if err != nil {
		return "", fmt.Errorf("parse server url: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", fmt.Errorf("server url must start with http:// or https://")
	}
	if strings.TrimSpace(u.Host) == "" {
		return "", fmt.Errorf("server url host is required")
	}

	path := strings.TrimRight(u.Path, "/")
	switch {
	case path == "":
		u.Path = "/api/v1"
	case strings.HasSuffix(path, "/api/v1"):
		u.Path = path
	default:
		u.Path = path + "/api/v1"
	}

	u.RawQuery = ""
	u.Fragment = ""
	return strings.TrimRight(u.String(), "/"), nil
}
