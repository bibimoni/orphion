package bettermelon

import (
	"net/http"
	"testing"
	"time"

	"github.com/bibimoni/orphion/internal/common"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.APIURL != "https://api.bettermelon.ru" {
		t.Fatalf("APIURL = %q", cfg.APIURL)
	}
	if cfg.ProxyURL != "https://proxy.bettermelon.ru" {
		t.Fatalf("ProxyURL = %q", cfg.ProxyURL)
	}
	if cfg.Provider != common.BettermelonDefaultProvider {
		t.Fatalf("Provider = %q", cfg.Provider)
	}
	if cfg.UserAgent == "" {
		t.Fatal("UserAgent is empty")
	}
	if cfg.Timeout != 30*time.Second {
		t.Fatalf("Timeout = %s", cfg.Timeout)
	}
}

func TestNewClientValidatesConfig(t *testing.T) {
	tests := []struct {
		name string
		cfg  Config
	}{
		{name: "missing API URL", cfg: Config{ProxyURL: "https://proxy.example.com", Provider: "hianime"}},
		{name: "invalid API URL", cfg: Config{APIURL: "://bad", ProxyURL: "https://proxy.example.com", Provider: "hianime"}},
		{name: "invalid proxy URL", cfg: Config{APIURL: "https://api.example.com", ProxyURL: "relative", Provider: "hianime"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := NewClient(tt.cfg); err == nil {
				t.Fatal("NewClient() error = nil")
			}
		})
	}
}

func TestNewClientUsesInjectedHTTPClient(t *testing.T) {
	httpClient := &http.Client{}
	cfg := DefaultConfig()
	cfg.HTTPClient = httpClient

	client, err := NewClient(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if client.httpClient != httpClient {
		t.Fatal("NewClient() did not retain injected HTTP client")
	}
}

func TestNewClientUsesDefaultProvider(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Provider = ""

	client, err := NewClient(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if client.provider != common.BettermelonDefaultProvider {
		t.Fatalf("provider = %q, want %q", client.provider, common.BettermelonDefaultProvider)
	}
}
