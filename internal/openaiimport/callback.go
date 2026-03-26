package openaiimport

import (
	"context"
	"fmt"
	"html"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

const callbackPath = "/auth/callback"

type callbackResult struct {
	Code             string
	State            string
	Error            string
	ErrorDescription string
}

type callbackServer struct {
	redirectURL string
	server      *http.Server
	resultCh    chan callbackResult
	errCh       chan error
	once        sync.Once
}

func newCallbackServer(redirectURL string) (*callbackServer, error) {
	parsed, err := url.Parse(strings.TrimSpace(redirectURL))
	if err != nil {
		return nil, fmt.Errorf("parse redirect url: %w", err)
	}
	if parsed.Scheme != "http" {
		return nil, fmt.Errorf("redirect url must use http")
	}
	if strings.TrimSpace(parsed.Host) == "" {
		return nil, fmt.Errorf("redirect url host is required")
	}

	path := strings.TrimSpace(parsed.Path)
	if path == "" {
		path = callbackPath
	}

	s := &callbackServer{
		redirectURL: strings.TrimRight(parsed.String(), "/"),
		resultCh:    make(chan callbackResult, 1),
		errCh:       make(chan error, 1),
	}

	mux := http.NewServeMux()
	mux.HandleFunc(path, s.handleCallback)
	mux.HandleFunc("/", s.handleNotFound)

	s.server = &http.Server{
		Addr:              parsed.Host,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	return s, nil
}

func (s *callbackServer) start() error {
	ln, err := net.Listen("tcp", s.server.Addr)
	if err != nil {
		return err
	}

	go func() {
		err := s.server.Serve(ln)
		if err != nil && err != http.ErrServerClosed {
			select {
			case s.errCh <- err:
			default:
			}
		}
	}()

	return nil
}

func (s *callbackServer) wait(ctx context.Context) (callbackResult, error) {
	select {
	case result := <-s.resultCh:
		return result, nil
	case err := <-s.errCh:
		return callbackResult{}, err
	case <-ctx.Done():
		return callbackResult{}, ctx.Err()
	}
}

func (s *callbackServer) shutdown(ctx context.Context) error {
	return s.server.Shutdown(ctx)
}

func (s *callbackServer) handleCallback(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()

	if oauthErr := strings.TrimSpace(query.Get("error")); oauthErr != "" {
		result := callbackResult{
			Error:            oauthErr,
			ErrorDescription: strings.TrimSpace(query.Get("error_description")),
		}
		s.complete(result)
		writeHTML(w, http.StatusBadRequest, callbackHTMLPage("OAuth failed", result.errorMessage(), false))
		return
	}

	code := strings.TrimSpace(query.Get("code"))
	state := strings.TrimSpace(query.Get("state"))
	if code == "" || state == "" {
		writeHTML(w, http.StatusBadRequest, callbackHTMLPage("OAuth failed", "Missing code or state in callback.", false))
		return
	}

	s.complete(callbackResult{
		Code:  code,
		State: state,
	})
	writeHTML(w, http.StatusOK, callbackHTMLPage("OAuth complete", "Authorization succeeded. You can close this window and return to the terminal.", true))
}

func (s *callbackServer) handleNotFound(w http.ResponseWriter, _ *http.Request) {
	writeHTML(w, http.StatusNotFound, callbackHTMLPage("Not found", "This callback server only handles the OAuth callback path.", false))
}

func (s *callbackServer) complete(result callbackResult) {
	s.once.Do(func() {
		s.resultCh <- result
	})
}

func (r callbackResult) errorMessage() string {
	if strings.TrimSpace(r.ErrorDescription) != "" {
		return fmt.Sprintf("%s: %s", r.Error, r.ErrorDescription)
	}
	return r.Error
}

func callbackHTMLPage(title, message string, success bool) string {
	accent := "#166534"
	if !success {
		accent = "#991b1b"
	}

	return fmt.Sprintf(`<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>%s</title>
  <style>
    :root { color-scheme: light; }
    body {
      margin: 0;
      font-family: ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
      background: #f5f7fb;
      color: #0f172a;
      min-height: 100vh;
      display: grid;
      place-items: center;
    }
    main {
      width: min(540px, calc(100vw - 32px));
      padding: 28px;
      background: #ffffff;
      border: 1px solid #dbe3f0;
      border-radius: 18px;
      box-shadow: 0 20px 60px rgba(15, 23, 42, 0.08);
    }
    h1 {
      margin: 0 0 12px;
      font-size: 1.35rem;
      color: %s;
    }
    p {
      margin: 0;
      line-height: 1.55;
      color: #334155;
      white-space: pre-wrap;
      word-break: break-word;
    }
  </style>
</head>
<body>
  <main>
    <h1>%s</h1>
    <p>%s</p>
  </main>
</body>
</html>`,
		html.EscapeString(title),
		accent,
		html.EscapeString(title),
		html.EscapeString(message),
	)
}

func writeHTML(w http.ResponseWriter, status int, content string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write([]byte(content))
}
