package allanime

import (
	"net/http"
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.APIURL != "https://api.allanime.day/api" {
		t.Fatalf("APIURL = %q", cfg.APIURL)
	}
	if cfg.SiteURL != "https://youtu-chan.com" {
		t.Fatalf("SiteURL = %q", cfg.SiteURL)
	}
	if cfg.MediaURL != "https://allanime.day" {
		t.Fatalf("MediaURL = %q", cfg.MediaURL)
	}
	if cfg.UserAgent == "" {
		t.Fatal("UserAgent is empty")
	}
	if cfg.Timeout != 30*time.Second {
		t.Fatalf("Timeout = %s", cfg.Timeout)
	}
	if cfg.EpisodeQueryHash != "d405d0edd690624b66baba3068e0edc3ac90f1597d898a1ec8db4e5c43c00fec" {
		t.Fatalf("EpisodeQueryHash = %q", cfg.EpisodeQueryHash)
	}
}

func TestNewClientValidatesConfig(t *testing.T) {
	tests := []struct {
		name string
		cfg  Config
	}{
		{name: "missing API URL", cfg: Config{SiteURL: "https://example.com", MediaURL: "https://media.example.com"}},
		{name: "invalid API URL", cfg: Config{APIURL: "://bad", SiteURL: "https://example.com", MediaURL: "https://media.example.com"}},
		{name: "invalid site URL", cfg: Config{APIURL: "https://api.example.com", SiteURL: "relative", MediaURL: "https://media.example.com"}},
		{name: "invalid media URL", cfg: Config{APIURL: "https://api.example.com", SiteURL: "https://example.com", MediaURL: "relative"}},
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
