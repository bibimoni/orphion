// Package allanime implements the built-in AllAnime-derived provider.
package allanime

import (
	"fmt"
	"net/http"
	"net/url"
	"time"
)

const defaultUserAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10.15; rv:150.0) Gecko/20100101 Firefox/150.0"

// Config contains provider-owned upstream settings.
type Config struct {
	APIURL           string
	SiteURL          string
	MediaURL         string
	UserAgent        string
	EpisodeQueryHash string
	Timeout          time.Duration
	HTTPClient       *http.Client
}

// DefaultConfig returns the production AllAnime configuration.
func DefaultConfig() Config {
	return Config{
		APIURL:           "https://api.allanime.day/api",
		SiteURL:          "https://youtu-chan.com",
		MediaURL:         "https://allanime.day",
		UserAgent:        defaultUserAgent,
		EpisodeQueryHash: "d405d0edd690624b66baba3068e0edc3ac90f1597d898a1ec8db4e5c43c00fec",
		Timeout:          30 * time.Second,
	}
}

func parseEndpoint(name, raw string) (*url.URL, error) {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return nil, fmt.Errorf("invalid %s URL", name)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, fmt.Errorf("invalid %s URL scheme", name)
	}
	return u, nil
}
