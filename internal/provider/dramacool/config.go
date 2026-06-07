// Package dramacool implements the built-in DramaCool provider.
package dramacool

import (
	"fmt"
	"net/http"
	"net/url"
	"time"
)

const defaultUserAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10.15; rv:150.0) Gecko/20100101 Firefox/150.0"

// Config contains provider-owned upstream settings.
type Config struct {
	BaseURL    string
	APIURL     string // Xyra Stream API base URL (optional, for stream resolution)
	APIKey     string // Xyra Stream API key (optional)
	UserAgent  string
	Timeout    time.Duration
	HTTPClient *http.Client
}

// DefaultConfig returns the production DramaCool configuration.
func DefaultConfig() Config {
	return Config{
		BaseURL:   "https://dramacool.sh",
		APIURL:    "https://api.xyra.stream/v1/dramacool",
		UserAgent: defaultUserAgent,
		Timeout:   30 * time.Second,
	}
}

func parseBaseURL(raw string) (*url.URL, error) {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return nil, fmt.Errorf("invalid base URL")
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, fmt.Errorf("invalid base URL scheme")
	}
	return u, nil
}
