package openaiimport

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClientGenerateAndCreateAccount(t *testing.T) {
	var sawGenerate bool
	var sawCreate bool

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("x-api-key") != "admin-test-key" {
			t.Fatalf("unexpected api key: %q", r.Header.Get("x-api-key"))
		}

		switch r.URL.Path {
		case "/api/v1/admin/openai/generate-auth-url":
			sawGenerate = true
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"code":0,"message":"ok","data":{"auth_url":"https://auth.openai.com/example","session_id":"sess-123"}}`))
		case "/api/v1/admin/openai/create-from-oauth":
			sawCreate = true

			var payload createAccountFromOAuthRequest
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatalf("decode payload: %v", err)
			}
			if payload.SessionID != "sess-123" || payload.Code != "code-123" || payload.State != "state-123" {
				t.Fatalf("unexpected create payload: %+v", payload)
			}

			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"code":0,"message":"ok","data":{"id":42,"name":"supplier-account","platform":"openai","type":"oauth","status":"active","concurrency":1,"priority":0,"group_ids":[7]}}`))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer srv.Close()

	client, err := newClient(srv.URL, "admin-test-key", platformOpenAI)
	if err != nil {
		t.Fatalf("newClient() error = %v", err)
	}

	auth, err := client.generateAuthURL(context.Background(), generateAuthURLRequest{
		RedirectURI: defaultRedirectURL,
	})
	if err != nil {
		t.Fatalf("generateAuthURL() error = %v", err)
	}
	if auth.SessionID != "sess-123" {
		t.Fatalf("unexpected session id: %q", auth.SessionID)
	}

	acc, err := client.createAccountFromOAuth(context.Background(), createAccountFromOAuthRequest{
		SessionID:   auth.SessionID,
		Code:        "code-123",
		State:       "state-123",
		RedirectURI: defaultRedirectURL,
		Concurrency: 1,
		GroupIDs:    []int64{7},
	})
	if err != nil {
		t.Fatalf("createAccountFromOAuth() error = %v", err)
	}
	if acc.ID != 42 || acc.Name != "supplier-account" {
		t.Fatalf("unexpected account: %+v", acc)
	}
	if !sawGenerate || !sawCreate {
		t.Fatalf("expected both generate and create requests, got generate=%v create=%v", sawGenerate, sawCreate)
	}
}

func TestNormalizeAPIBaseURL(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{name: "append api prefix", input: "https://example.com", want: "https://example.com/api/v1"},
		{name: "keep api prefix", input: "https://example.com/api/v1", want: "https://example.com/api/v1"},
		{name: "keep nested prefix", input: "https://example.com/sub2api/api/v1/", want: "https://example.com/sub2api/api/v1"},
		{name: "reject missing scheme", input: "example.com", wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := normalizeAPIBaseURL(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("normalizeAPIBaseURL(%q) expected error", tc.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("normalizeAPIBaseURL(%q) error = %v", tc.input, err)
			}
			if got != tc.want {
				t.Fatalf("normalizeAPIBaseURL(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}
