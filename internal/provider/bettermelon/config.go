// Package bettermelon implements the Bettermelon streaming provider.
package bettermelon

import (
	"errors"
	"net/http"
	"net/url"
	"time"

	"github.com/bibimoni/orphion/internal/common"
)

// Config contains provider-owned upstream settings.
type Config struct {
	APIURL     string
	ProxyURL   string
	UserAgent  string
	Timeout    time.Duration
	HTTPClient *http.Client
	Provider   string // upstream provider: "hianime", "animekai", "kickassanime", "anikoto"
}

// DefaultConfig returns the production Bettermelon configuration.
func DefaultConfig() Config {
	return Config{
		APIURL:    common.BettermelonAPIURL,
		ProxyURL:  common.BettermelonProxyURL,
		UserAgent: common.DefaultUserAgent,
		Timeout:   common.DefaultHTTPTimeout,
		Provider:  common.BettermelonDefaultProvider,
	}
}

func parseEndpoint(name, raw string) (*url.URL, error) {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return nil, errors.New("invalid " + name + " URL")
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, errors.New("invalid " + name + " URL scheme")
	}
	return u, nil
}
