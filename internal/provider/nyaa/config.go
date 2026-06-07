// Package nyaa implements the built-in Nyaa.si torrent provider.
package nyaa

import (
	"fmt"
	"net/http"
	"net/url"
	"time"
)

const defaultUserAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10.15; rv:150.0) Gecko/20100101 Firefox/150.0"

// Category constants for Nyaa.si search filtering.
const (
	// CategoryLiveActionSubbed is the Nyaa category for English-translated live action.
	CategoryLiveActionSubbed = "4_3"
	// CategoryLiveActionRaw is the Nyaa category for raw live action.
	CategoryLiveActionRaw = "4_4"
	// CategoryAnimeSubbed is the Nyaa category for English-translated anime.
	CategoryAnimeSubbed = "1_2"
	// CategoryAnimeRaw is the Nyaa category for raw anime.
	CategoryAnimeRaw = "1_4"
)

// Standard Nyaa trackers included in generated magnet URIs.
var defaultTrackers = []string{
	"udp://open.stealth.si:80/announce",
	"udp://tracker.opentrackr.org:1337/announce",
	"udp://exodus.desync.com:6969/announce",
	"udp://tracker.torrent.eu.org:451/announce",
	"http://nyaa.tracker.wf:7777/announce",
}

// Config contains provider-owned upstream settings.
type Config struct {
	BaseURL    string
	Category   string // Nyaa category filter (e.g. "4_3" for subbed live action)
	UserAgent  string
	Timeout    time.Duration
	HTTPClient *http.Client
}

// DefaultConfig returns the production Nyaa configuration.
func DefaultConfig() Config {
	return Config{
		BaseURL:   "https://nyaa.si",
		Category:  CategoryLiveActionSubbed,
		UserAgent: defaultUserAgent,
		Timeout:   60 * time.Second,
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
