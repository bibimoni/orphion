package subdl

import (
	"net/http"
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.SiteURL != "https://subdl.com" {
		t.Fatalf("SiteURL = %q", cfg.SiteURL)
	}
	if cfg.DownloadURL != "https://dl.subdl.com" {
		t.Fatalf("DownloadURL = %q", cfg.DownloadURL)
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
		{name: "missing site URL", cfg: Config{SiteURL: "", DownloadURL: "https://dl.subdl.com"}},
		{name: "invalid site URL", cfg: Config{SiteURL: "://bad", DownloadURL: "https://dl.subdl.com"}},
		{name: "missing download URL", cfg: Config{SiteURL: "https://subdl.com", DownloadURL: ""}},
		{name: "invalid download URL", cfg: Config{SiteURL: "https://subdl.com", DownloadURL: "relative"}},
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
