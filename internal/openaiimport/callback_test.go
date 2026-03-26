package openaiimport

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"testing"
	"time"
)

func TestCallbackServerReceivesCodeAndState(t *testing.T) {
	redirectURL := testRedirectURL(t)
	server, err := newCallbackServer(redirectURL)
	if err != nil {
		t.Fatalf("newCallbackServer() error = %v", err)
	}
	if err := server.start(); err != nil {
		t.Fatalf("start() error = %v", err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = server.shutdown(ctx)
	}()

	resp, err := http.Get(redirectURL + "?code=abc123&state=state456")
	if err != nil {
		t.Fatalf("GET callback: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected status code: %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read response body: %v", err)
	}
	if len(body) == 0 {
		t.Fatal("expected callback page body")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	result, err := server.wait(ctx)
	if err != nil {
		t.Fatalf("wait() error = %v", err)
	}
	if result.Code != "abc123" || result.State != "state456" {
		t.Fatalf("unexpected callback result: %+v", result)
	}
}

func testRedirectURL(t *testing.T) string {
	t.Helper()

	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("allocate port: %v", err)
	}
	addr := l.Addr().String()
	if err := l.Close(); err != nil {
		t.Fatalf("close listener: %v", err)
	}
	return fmt.Sprintf("http://%s%s", addr, callbackPath)
}
