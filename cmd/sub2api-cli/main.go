package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"

	"github.com/Wei-Shaw/sub2api-cli/internal/openaiimport"
)

type int64SliceFlag []int64

func (f *int64SliceFlag) String() string {
	if f == nil || len(*f) == 0 {
		return ""
	}
	parts := make([]string, 0, len(*f))
	for _, id := range *f {
		parts = append(parts, strconv.FormatInt(id, 10))
	}
	return strings.Join(parts, ",")
}

func (f *int64SliceFlag) Set(value string) error {
	for _, part := range strings.Split(value, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		id, err := strconv.ParseInt(part, 10, 64)
		if err != nil {
			return fmt.Errorf("invalid group id %q: %w", part, err)
		}
		*f = append(*f, id)
	}
	return nil
}

func main() {
	var (
		groupIDs    int64SliceFlag
		serverURL   = flag.String("server", firstNonEmpty(os.Getenv("SUB2API_SERVER"), os.Getenv("SUB2API_BASE_URL")), "Sub2API server base URL, e.g. https://example.com")
		apiKey      = flag.String("api-key", os.Getenv("SUB2API_ADMIN_API_KEY"), "Sub2API admin x-api-key")
		platform    = flag.String("platform", "openai", "OAuth platform: openai or sora")
		name        = flag.String("name", "", "Account name override; defaults to OAuth email")
		redirectURL = flag.String("redirect-url", "http://localhost:1455/auth/callback", "Local OAuth callback URL")
		proxyID     = flag.Int64("proxy-id", -1, "Optional Sub2API proxy ID")
		concurrency = flag.Int("concurrency", 10, "Account concurrency")
		priority    = flag.Int("priority", 0, "Account priority")
		noOpen      = flag.Bool("no-open", false, "Do not auto-open the browser")
	)
	flag.Var(&groupIDs, "group-id", "Group ID to bind the account to; repeat or pass comma-separated values")
	flag.Parse()

	cfg := openaiimport.Config{
		ServerURL:   strings.TrimSpace(*serverURL),
		APIKey:      strings.TrimSpace(*apiKey),
		Platform:    strings.TrimSpace(*platform),
		Name:        strings.TrimSpace(*name),
		Concurrency: *concurrency,
		Priority:    *priority,
		GroupIDs:    groupIDs,
		RedirectURL: strings.TrimSpace(*redirectURL),
		NoOpen:      *noOpen,
		StatusOut:   os.Stderr,
	}
	if *proxyID >= 0 {
		cfg.ProxyID = proxyID
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	result, err := openaiimport.Run(ctx, cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "sub2api-cli failed: %v\n", err)
		os.Exit(1)
	}

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(result); err != nil {
		fmt.Fprintf(os.Stderr, "encode result: %v\n", err)
		os.Exit(1)
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
