package jimaku

import (
	"net/http"
	"testing"
	"time"

	"github.com/distiled/orphion/internal/common"
)

func TestDefaultConfigUsesCommonConstants(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.BaseURL != common.JimakuSiteURL {
		t.Errorf("BaseURL = %q, want %q", cfg.BaseURL, common.JimakuSiteURL)
	}
	if cfg.UserAgent != common.DefaultUserAgent {
		t.Errorf("UserAgent = %q, want %q", cfg.UserAgent, common.DefaultUserAgent)
	}
	if cfg.Timeout != common.JimakuTimeout {
		t.Errorf("Timeout = %v, want %v", cfg.Timeout, common.JimakuTimeout)
	}
}

func TestDefaultConfigValuesAreNonZero(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.BaseURL == "" {
		t.Error("BaseURL is empty")
	}
	if cfg.UserAgent == "" {
		t.Error("UserAgent is empty")
	}
	if cfg.Timeout <= 0 {
		t.Errorf("Timeout = %v, want positive", cfg.Timeout)
	}
}

func TestDefaultHTTPClientReturnsConfiguredTimeout(t *testing.T) {
	cfg := Config{
		BaseURL:   "https://example.com",
		UserAgent: "test",
		Timeout:   10 * time.Second,
	}
	client := cfg.defaultHTTPClient()
	if client == nil {
		t.Fatal("defaultHTTPClient() returned nil")
	}
	if client.Timeout != 10*time.Second {
		t.Errorf("defaultHTTPClient Timeout = %v, want 10s", client.Timeout)
	}
}

func TestDefaultHTTPClientLimitsRedirects(t *testing.T) {
	cfg := DefaultConfig()
	client := cfg.defaultHTTPClient()
	if client.CheckRedirect == nil {
		t.Error("CheckRedirect is nil, want redirect limiter")
	}
}

func TestCustomHTTPClientOverridesDefault(t *testing.T) {
	custom := &http.Client{Timeout: 99 * time.Second}
	cfg := Config{
		BaseURL:    "https://example.com",
		UserAgent:  "test",
		Timeout:    5 * time.Second,
		HTTPClient: custom,
	}
	c, err := NewClient(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if c.client != custom {
		t.Error("NewClient did not use the custom HTTPClient")
	}
}

func TestNilHTTPClientUsesDefault(t *testing.T) {
	cfg := Config{
		BaseURL:   "https://example.com",
		UserAgent: "test",
		Timeout:   5 * time.Second,
	}
	c, err := NewClient(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if c.client == nil {
		t.Fatal("client is nil")
	}
	if c.client.Timeout != 5*time.Second {
		t.Errorf("client Timeout = %v, want 5s", c.client.Timeout)
	}
}
