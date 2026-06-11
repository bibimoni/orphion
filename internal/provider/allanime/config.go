// Package allanime implements the built-in AllAnime-derived provider.
package allanime

import (
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/bibimoni/orphion/internal/common"
)

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
		APIURL:           common.AllAnimeAPIURL,
		SiteURL:          common.AllAnimeSiteURL,
		MediaURL:         common.AllAnimeMediaURL,
		UserAgent:        common.DefaultUserAgent,
		EpisodeQueryHash: common.AllAnimeEpisodeQueryHash,
		Timeout:          common.DefaultHTTPTimeout,
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
